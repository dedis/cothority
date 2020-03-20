import { Message } from "protobufjs";
import { Observable } from "rxjs";
import Log from "../log";
import { IConnection } from "./nodes";
import { BrowserWebSocketAdapter, WebSocketAdapter } from "./websocket-adapter";

let factory: (path: string) => WebSocketAdapter = (path: string) => new BrowserWebSocketAdapter(path);

/**
 * Set the websocket generator. The default one is compatible
 * with browsers and nodejs.
 * @param generator A function taking a path and creating a websocket adapter instance
 */
export function setFactory(generator: (path: string) => WebSocketAdapter): void {
    factory = generator;
}

/**
 * Single peer connection to one single node.
 */
export class WebSocketConnection implements IConnection {
    private readonly url: URL;
    private readonly service: string;
    private timeout: number;

    /**
     * @param addr      Absolute address of the distant peer
     * @param service   Name of the service to reach
     */
    constructor(addr: string | URL, service: string) {
        let url: URL;
        if (typeof addr === "string") {
            url = new URL(addr);
        } else {
            url = addr;
        }
        // We want any pathname to contain a "/" at the end. This is motivated
        // by the fact that URL will not allow you to have an empty pathname,
        // which will always equal to "/" if there isn't any
        if (url.pathname.slice(-1) !== "/") {
            url.pathname = url.pathname + "/";
        }
        if (url.username !== "" || url.password !== "") {
            throw new Error("addr contains authentication, which is not supported");
        }
        if (url.search !== "" || url.hash !== "") {
            throw new Error("addr contains more data than the origin");
        }

        if (typeof globalThis !== "undefined" && typeof globalThis.location !== "undefined") {
            if (globalThis.location.protocol === "https:") {
                url.protocol = "wss";
            }
        }

        this.service = service;
        this.timeout = 30 * 1000; // 30s by default
        this.url = url;
    }

    /** @inheritdoc */
    getURL(): string {
        // Retro compatibility: this.url always ends with a slash, but the old
        // behavior needs no trailing slash
        return this.url.href.slice(0, -1);
    }

    /** @inheritdoc */
    setTimeout(value: number): void {
        this.timeout = value;
    }

    /** @inheritdoc */
    async send<T extends Message>(message: Message, reply: typeof Message): Promise<T> {
        return new Promise((complete, error) => {
            this.sendStream(message, reply).subscribe({
                complete,
                error,
                next: ([m, ws]) => {
                    complete(m as T);
                    ws.close(1000);
                },
            });
        });
    }

    copy(service: string): IConnection {
        return new WebSocketConnection(this.url, service);
    }

    /**
     * @deprecated - use directly a RosterWSConnection if you want that
     * @param p
     */
    setParallel(p: number): void {
        if (p > 1) {
            throw new Error("Single connection doesn't support more than one parallel");
        }
    }

    /** @inheritdoc */
    sendStream<T extends Message>(message: Message, reply: typeof Message):
        Observable<[T, WebSocketAdapter]> {

        if (!message.$type) {
            throw new Error(`message "${message.constructor.name}" is not registered`);
        }
        if (!reply.$type) {
            throw new Error(`message "${reply.constructor.name}" is not registered`);
        }

        return new Observable((sub) => {
            const url = new URL(this.url.href);
            url.pathname += `${this.service}/${message.$type.name.replace(/.*\./, "")}`;
            Log.lvl4(`Socket: new WebSocket(${url.href})`);
            const ws = factory(url.href);
            const bytes = Buffer.from(message.$type.encode(message).finish());
            const timer = setTimeout(() => ws.close(1000, "timeout"), this.timeout);

            ws.onOpen(() => {
                Log.lvl3("Sending message to", url.href);
                ws.send(bytes);
            });

            ws.onMessage((data: Buffer) => {
                clearTimeout(timer);
                const buf = Buffer.from(data);
                Log.lvl4("Getting message with length:", buf.length);

                try {
                    const ret = reply.decode(buf) as T;
                    sub.next([ret, ws]);
                } catch (err) {
                    sub.error(err);
                }
            });

            ws.onClose((code: number, reason: string) => {
                // nativescript-websocket on iOS doesn't return error-code 1002 in case of error, but sets the 'reason'
                // to non-null in case of error.
                if (code !== 1000 || (reason && reason !== "")) {
                    sub.error(new Error(reason));
                } else {
                    sub.complete();
                }
            });

            ws.onError((err: Error) => {
                clearTimeout(timer);

                if (err !== undefined) {
                    sub.error(new Error(`error in websocket ${url.href}: ${err.message}`));
                } else {
                    sub.complete();
                }
            });

        });
    }
}

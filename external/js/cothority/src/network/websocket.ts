import { Message } from "protobufjs";
import { Observable } from "rxjs";
import Log from "../log";
import { IConnection } from "./nodes";
import { Roster } from "./proto";
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
            url.pathname = `${url.pathname}/`;
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
        // 50s by default. Onet will close a connection after 1 minute.
        this.timeout = 50 * 1000;
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
            url.pathname = `${url.pathname}${this.service}/${message.$type.name.replace(/.*\./, "")}`;
            Log.lvl4(`Socket: new WebSocket(${url.href})`);
            const ws = factory(url.href);
            const bytes = Buffer.from(message.$type.encode(message).finish());
            let timer = setTimeout(() => ws.close(1000, "timeout"), this.timeout);

            ws.onOpen(() => {
                Log.lvl3("Sending message to", url.href);
                ws.send(bytes);
            });

            ws.onMessage((data: Buffer) => {

                // clear the timer and set a new one
                clearTimeout(timer);
                timer = setTimeout(() => ws.close(1000, "timeout"), this.timeout);

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
                    let err = "Unknown close error";
                    switch (code) {
                        case 1001:
                            err = `Endpoint is "going away": server going down or a browser
                            having navigated away from the page.`;
                            break;
                        case 1002:
                            err = `Endpoint terminated the connection due to a protocol error.`;
                            break;
                        case 1003:
                            err = `Endpoint terminated the connection
                            because it has received a type of data it cannot accept (e.g., an
                            endpoint that understands only text data MAY send this if it
                            receives a binary message).`;
                            break;
                        case 1004:
                            err = `Reserved. The specific meaning might be defined in the future.`;
                            break;
                        case 1005:
                            err = `No status code was actually present.`;
                            break;
                        case 1006:
                            err = `Connection was closed abnormally, e.g., without sending or
                            receiving a Close control frame.`;
                            break;
                        case 1007:
                            err = `Endpoint terminated the connection
                            because it has received data within a message that was not
                            consistent with the type of the message (e.g., non-UTF-8 [RFC3629]
                            data within a text message).`;
                            break;
                        case 1008:
                            err = `Endpoint terminated the connection
                            because it has received a message that violates its policy.`;
                            break;
                        case 1009:
                            err = `Endpoint terminated the connection
                            because it has received a message that is too big for it to
                            process.`;
                            break;
                        case 1010:
                            err = `Endpoint terminated the
                            connection because it has expected the server to negotiate one or
                            more extension, but the server didn't return them in the response
                            message of the WebSocket handshake.`;
                            break;
                        case 1011:
                            err = `Server terminated the connection because
                            it encountered an unexpected condition that prevented it from
                            fulfilling the request.`;
                            break;
                        case 1015:
                            err = `Connection was closed due to a failure to perform a TLS handshake
                            (e.g., the server certificate can't be verified).`;
                            break;
                    }
                    if (reason && reason !== "") {
                        err += " Reason: " + reason;
                    }

                    sub.error(new Error(err.replace(/\n/g, "").
                        replace(/ +/g, " ")));
                } else {
                    sub.complete();
                }
            });

            ws.onError((err: Error) => {
                clearTimeout(timer);

                if (err !== undefined) {
                    sub.error(new Error(`error in websocket ${url.href}: ${err.message}`));
                } else {
                    sub.error(new Error("unknown reason"));
                }
            });

        });
    }
}

/**
 * Single peer connection that reaches only the leader of the roster
 */
export class LeaderConnection extends WebSocketConnection {
    /**
     * @param roster    The roster to use
     * @param service   The name of the service
     */
    constructor(roster: Roster, service: string) {
        if (roster.list.length === 0) {
            throw new Error("Roster should have at least one node");
        }

        super(roster.list[0].getWebSocketAddress(), service);
    }
}

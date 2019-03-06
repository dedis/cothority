import { WebSocketAdapter } from "../../src/network/websocket-adapter";

export default class TestWebSocket extends WebSocketAdapter {
    isOpen = false;
    isClosed = false;
    data: Buffer;
    error: Error;
    code: number;
    sent: Buffer;
    onclose: (code: number, reason: string) => void;

    constructor(data: Buffer, error: Error, code: number) {
        super("");

        this.data = data;
        this.error = error;
        this.code = code;
        this.isOpen = true;
    }

    onOpen(callback: () => void): void {
        callback();
    }

    onMessage(callback: (data: Buffer) => void): void {
        if (this.data) {
            callback(this.data);
        }
    }

    onClose(callback: (code: number, reason: string) => void): void {
        if (this.code) {
            callback(this.code, "reason to close");
        } else {
            this.onclose = callback;
        }
    }

    onError(callback: (err: Error) => void): void {
        if (this.error) {
            callback(this.error);
        }
    }

    send(bytes: Buffer): void {
        this.sent = bytes;
    }

    close(code: number, reason = ""): void {
        this.isClosed = true;

        if (this.onclose) {
            this.onclose(code, reason);
        }
    }
}

import { WebSocketAdapter } from '../../src/network/websocket-adapter';

export default class TestWebSocket extends WebSocketAdapter {
    public isOpen = false;
    public isClosed = false;
    public data: Buffer;
    public error: Error;
    public code: number;
    public sent: Buffer;

    constructor(data: Buffer, error: Error, code: number = 1000) {
        super('');

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
        callback(this.code, 'reason to close');
    }

    onError(callback: (err: Error) => void): void {
        if (this.error) {
            callback(this.error);
        }
    }

    send(bytes: Buffer): void {
        this.sent = bytes;
    }

    close(code: number): void {
        this.isClosed = true;
    }
}

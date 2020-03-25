import { Message } from "protobufjs/light";
import {
    BrowserWebSocketAdapter,
    LeaderConnection,
    RosterWSConnection,
    setFactory,
    WebSocketConnection,
} from "../../src/network";
import { Roster, ServerIdentity } from "../../src/network/proto";
import { ROSTER } from "../support/conondes";
import TestWebSocket from "./websocket-test-adapter";

class UnregisteredMessage extends Message<UnregisteredMessage> {
}

describe("WebSocketAdapter Tests", () => {
    afterAll(() => {
        setFactory((path: string) => new BrowserWebSocketAdapter(path));

        globalThis.location = {
            ...globalThis.location,
            protocol: "",
        };
    });

    it("should send and receive data", async () => {
        const ret = Buffer.from(Roster.encode(new Roster()).finish());
        setFactory(() => new TestWebSocket(ret, null, 1000));
        const conn = new WebSocketConnection("http://example.com", "");
        const msg = new Roster();

        await expectAsync(conn.send(msg, Roster)).toBeResolved();
    });

    it("should throw an error when code is not 1000", async () => {
        setFactory(() => new TestWebSocket(null, null, 1001));

        const conn = new WebSocketConnection("http://example.com", "");
        const msg = new Roster();

        await expectAsync(conn.send(msg, Roster)).toBeRejectedWith(new Error("reason to close"));
    });

    it("should timeout when no message is sent back", async () => {
        setFactory(() => new TestWebSocket(null, null, null));

        const conn = new WebSocketConnection("http://example.com", "");
        conn.setTimeout(200);
        const msg = new Roster();

        await expectAsync(conn.send(msg, Roster)).toBeRejectedWith(new Error("timeout"));
    });

    it("should throw on protobuf error", async () => {
        setFactory(() => new TestWebSocket(Buffer.from([1, 2, 3]), null, 1000));

        const conn = new WebSocketConnection("http://example.com", "");
        const msg = new Roster();

        await expectAsync(conn.send(msg, Roster)).toBeRejected();
    });

    it("should reject unregistered messages", async () => {
        const conn = new WebSocketConnection("http://example.com", "");

        await expectAsync(conn.send(new UnregisteredMessage(), UnregisteredMessage)).toBeRejected();
        await expectAsync(conn.send(new Roster(), UnregisteredMessage)).toBeRejected();
    });

    it("should try the roster", async () => {
        const ret = Buffer.from(Roster.encode(new Roster()).finish());
        setFactory((path: string) => {
            if (path === "a") {
                return new TestWebSocket(null, new Error("random"), 1000);
            } else {
                return new TestWebSocket(ret, null, 1000);
            }
        });
        const roster = new Roster({
            list: [
                new ServerIdentity({
                    address: "tls://a:1234",
                    public: ROSTER.list[0].public,
                }),
                new ServerIdentity({
                    address: "tls://b:1234",
                    public: ROSTER.list[0].public,
                }),
            ],
        });

        const conn = new RosterWSConnection(roster, "");
        const reply = await conn.send(roster, Roster);

        expect(reply instanceof Roster).toBeTruthy();
    });

    it("should fail to try all conodes", async () => {
        setFactory(() => new TestWebSocket(null, new Error(), 1000));
        const roster = new Roster({
            list: [
                new ServerIdentity({
                    address: "tls://a:1234",
                    public: ROSTER.list[0].public,
                }),
                new ServerIdentity({
                    address: "tls://b:1234",
                    public: ROSTER.list[0].public,
                }),
            ],
        });

        const conn = new RosterWSConnection(roster, "");

        await expectAsync(conn.send(roster, Roster)).toBeRejected();
    });

    it("should send a request to the leader", async () => {
        const roster = new Roster({
            list: [
                new ServerIdentity({
                    address: "tls://a:1234",
                    public: ROSTER.list[0].public,
                }),
                new ServerIdentity({
                    address: "tls://b:1234",
                    public: ROSTER.list[0].public,
                }),
            ],
        });

        const conn = new LeaderConnection(roster, "");
        expect(conn.getURL()).toBe("ws://a:1235");

        expect(() => new LeaderConnection(new Roster(), "")).toThrow();
    });

    it("should switch to wss in https context", async () => {
        const conn = new WebSocketConnection("ws://a:1234", "");
        expect(conn.getURL()).toBe("ws://a:1234");

        const oldLocation = globalThis.location;
        try {
            globalThis.location = {
                ...globalThis.location,
                protocol: "https:",
            };

            const conn2 = new WebSocketConnection("ws://a:1234", "");
            expect(conn2.getURL()).toBe("wss://a:1234");
        } finally {
            globalThis.location = oldLocation;
        }
    });

    it("should support having an URL with a pathname", async () => {
        const conn = new WebSocketConnection("ws://a:1234/path", "");
        expect(conn.getURL()).toBe("ws://a:1234/path");
    });

});

describe("WebSocketAdapter Tests with sendStream", () => {
    afterAll(() => {
        setFactory((path: string) => new BrowserWebSocketAdapter(path));

        globalThis.location = {
            ...globalThis.location,
            protocol: "",
        };
    });

    it("should send and receive data", async (done) => {
        const ret = Buffer.from(Roster.encode(new Roster()).finish());
        setFactory(() => new TestWebSocket(ret, null, null));
        const conn = new WebSocketConnection("http://example.com", "");
        const msg = new Roster();

        conn.sendStream(msg, Roster).subscribe({
            complete: () => {
                throw new Error("should not complete");
            },
            error: () => {
                throw new Error("should not get error");
            },
            next: () => {
                done();
            },
        });
    });

    it("should throw an error when code is not 1000", async (done) => {
        setFactory(() => new TestWebSocket(null, null, 1001));

        const conn = new WebSocketConnection("http://example.com", "");
        const msg = new Roster();

        conn.sendStream(msg, Roster).subscribe({
            complete: () => {
                throw new Error("should not complete");
            },
            error: (err: Error) => {
                expect(err).toEqual(new Error("reason to close"));
                done();
            },
            next: () => {
                throw new Error("should not get value");
            },
        });
    });

    it("should timeout when no message is sent back", async (done) => {
        setFactory(() => new TestWebSocket(null, null, null));

        const conn = new WebSocketConnection("http://example.com", "");
        conn.setTimeout(200);
        const msg = new Roster();

        conn.sendStream(msg, Roster).subscribe({
            complete: () => {
                throw new Error("should not complete");
            },
            error: (err: Error) => {
                expect(err).toEqual(new Error("timeout"));
                done();
            },
            next: () => {
                throw new Error("should not get value");
            },
        });
    });

    it("should throw on protobuf error", async (done) => {
        setFactory(() => new TestWebSocket(Buffer.from([1, 2, 3]), null, 1000));

        const conn = new WebSocketConnection("http://example.com", "");

        conn.sendStream(new Roster(), Roster).subscribe({
            complete: () => {
                throw new Error("shouldn't complete");
            },
            error: () => done(),
            next: () => {
                throw new Error("should not get value");
            },
        });
    });

    it("should reject unregistered messages and reply", () => {
        const conn = new WebSocketConnection("http://example.com", "");
        expect(() => conn.sendStream(new UnregisteredMessage(), UnregisteredMessage)).toThrowError();
    });

    it("should reject unregistered reply", () => {
        const conn = new WebSocketConnection("http://example.com", "");
        expect(() => conn.sendStream(new Roster(), UnregisteredMessage)).toThrowError();
    });

});

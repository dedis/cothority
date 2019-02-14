import { Message } from "protobufjs";
import { LeaderConnection, RosterWSConnection, setFactory, WebSocketConnection } from "../../src/network/connection";
import { Roster, ServerIdentity } from "../../src/network/proto";
import { BrowserWebSocketAdapter } from "../../src/network/websocket-adapter";
import { ROSTER } from "../support/conondes";
import TestWebSocket from "./websocket-test-adapter";

class UnregisteredMessage extends Message<UnregisteredMessage> {}

describe("WebSocketAdapter Tests", () => {
    afterAll(() => {
        setFactory((path: string) => new BrowserWebSocketAdapter(path));
    });

    it("should send and receive data", async () => {
        const ret = Buffer.from(Roster.encode(new Roster()).finish());
        setFactory(() => new TestWebSocket(ret, null));
        const conn = new WebSocketConnection("", "");
        const msg = new Roster();

        expectAsync(conn.send(msg, Roster)).toBeResolved();
    });

    it("should throw an error when code is not 1000", async () => {
        setFactory(() => new TestWebSocket(null, null, 1001));

        const conn = new WebSocketConnection("", "");
        const msg = new Roster();

        expectAsync(conn.send(msg, Roster)).toBeRejectedWith("reason to close");
    });

    it("should throw on protobuf error", async () => {
        setFactory(() => new TestWebSocket(Buffer.from([1, 2, 3]), null));

        const conn = new WebSocketConnection("", "");
        const msg = new Roster();

        expectAsync(conn.send(msg, Roster)).toBeRejected();
    });

    it("should reject unregistered messages", async () => {
        const conn = new WebSocketConnection("", "");

        expectAsync(conn.send(new UnregisteredMessage(), UnregisteredMessage)).toBeRejected();
        expectAsync(conn.send(new Roster(), UnregisteredMessage)).toBeRejected();
    });

    it("should try the roster", async () => {
        const ret = Buffer.from(Roster.encode(new Roster()).finish());
        setFactory((path: string) => {
            if (path === "a") {
                return new TestWebSocket(null, new Error("random"));
            } else {
                return new TestWebSocket(ret, null);
            }
        });
        const roster = new Roster({
            list: [
                new ServerIdentity({ address: "a", public: ROSTER.list[0].public }),
                new ServerIdentity({ address: "b", public: ROSTER.list[0].public }),
            ],
        });

        const conn = new RosterWSConnection(roster, "");
        const reply = await conn.send(roster, Roster);

        expect(reply instanceof Roster).toBeTruthy();
    });

    it("should fail to try all conodes", async () => {
        setFactory(() => new TestWebSocket(null, new Error()));
        const roster = new Roster({
            list: [
                new ServerIdentity({ address: "a", public: ROSTER.list[0].public }),
                new ServerIdentity({ address: "b", public: ROSTER.list[0].public }),
            ],
        });

        const conn = new RosterWSConnection(roster, "");

        expectAsync(conn.send(roster, Roster)).toBeRejected();
    });

    it("should send a request to the leader", async () => {
        const roster = new Roster({
            list: [
                new ServerIdentity({ address: "a", public: ROSTER.list[0].public }),
                new ServerIdentity({ address: "b", public: ROSTER.list[0].public }),
            ],
        });

        const conn = new LeaderConnection(roster, "");
        expect(conn.getURL()).toBe("a");

        expect(() => new LeaderConnection(new Roster(), "")).toThrow();
    });
});

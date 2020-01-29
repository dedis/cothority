import ByzCoinRPC from "../../src/byzcoin/byzcoin-rpc";
import { PaginateRequest, PaginateResponse } from "../../src/byzcoin/proto/stream";
import { WebSocketAdapter } from "../../src/network";
import { WebSocketConnection } from "../../src/network/connection";
import { SkipchainRPC } from "../../src/skipchain";
import { BLOCK_INTERVAL, ROSTER, SIGNER, startConodes } from "../support/conondes";

fdescribe("Stream Tests", () => {
    const roster = ROSTER.slice(0, 4);
    let originalTimeout: number;

    beforeAll(async () => {
        await startConodes();
        originalTimeout = jasmine.DEFAULT_TIMEOUT_INTERVAL;
        jasmine.DEFAULT_TIMEOUT_INTERVAL = 2000;
    });

    it("should send and receive data", async () => {
        const darc = ByzCoinRPC.makeGenesisDarc([SIGNER], roster);
        const rpc = await ByzCoinRPC.newByzCoinRPC(roster, darc, BLOCK_INTERVAL);

        const conn = new WebSocketConnection(roster.list[0].getWebSocketAddress(), SkipchainRPC.serviceName);

        const msg = new PaginateRequest({startid: rpc.genesisID, pagesize: 1, numpages: 1});

        const foo = {
            // tslint:disable-next-line:no-empty
            onClose: (code: number, reason: string) => {
                // tslint:disable-next-line
                console.log(">>>>>> on close", code, reason)
                // done();
            },
            // tslint:disable-next-line:no-empty
            onError: (err: Error) => {
                // tslint:disable-next-line
                console.log(">>>>> on error", err)
                // done();
            },
            // tslint:disable-next-line:no-empty
            onMessage: (message: PaginateResponse, ws: WebSocketAdapter) => {
                // done();
                expect(message.blocks.length).toEqual(1);
            },
        };

        spyOn(foo, "onClose");
        spyOn(foo, "onError");
        spyOn(foo, "onMessage");
        expect(foo.onClose).not.toHaveBeenCalled();
        expect(foo.onError).not.toHaveBeenCalled();
        expect(foo.onMessage).toHaveBeenCalled();

        conn.sendStream<PaginateResponse>(msg, PaginateResponse, foo.onMessage, foo.onClose, foo.onError);
    });

    afterEach( () => {
        jasmine.DEFAULT_TIMEOUT_INTERVAL = originalTimeout;
    });
});

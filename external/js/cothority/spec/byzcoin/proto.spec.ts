import Long from "long";
import ClientTransaction from "../../src/byzcoin/client-transaction";
import { DataBody, DataHeader, TxResult } from "../../src/byzcoin/proto";
import {
    AddTxRequest, CreateGenesisBlock,
} from "../../src/byzcoin/proto/requests";
import Darc from "../../src/darc/darc";

describe("ByzCoin Proto Tests", () => {
    it("should handle create genesis block messages", () => {
        const req = new CreateGenesisBlock({
            blockInterval: Long.fromNumber(1),
            genesisDarc: new Darc(),
            maxBlockSize: 42,
        });

        expect(req.genesisDarc).toBeDefined();
        expect(req.blockInterval.toNumber()).toBe(1);
        expect(req.maxBlockSize).toBe(42);

        expect(new CreateGenesisBlock()).toBeDefined();
    });

    it("should handle add tx request messages", () => {
        const req = new AddTxRequest({ skipchainID: Buffer.from([1, 2, 3]) });

        expect(req.skipchainID).toEqual(Buffer.from([1, 2, 3]));

        expect(new AddTxRequest()).toBeDefined();
    });

    it("should instantiate DataBody", () => {
        const obj = new DataBody();
        expect(obj.txResults).toEqual([]);

        const obj2 = new DataBody({ txResults: [] });
        // @ts-ignore
        expect(obj2.txresults).toEqual([]);
    });

    it("should instantiate DataHeader", () => {
        const dh = new DataHeader();
        expect(dh.trieRoot).toEqual(Buffer.from([]));

        const dh2 = new DataHeader({
            clientTransactionHash: Buffer.from([1, 2, 3]),
            stateChangeHash: Buffer.from([7, 8, 9]),
            trieRoot: Buffer.from([4, 5, 6]),
        });
        // @ts-ignore
        expect(dh2.clienttransactionhash).toEqual(Buffer.from([1, 2, 3]));
        // @ts-ignore
        expect(dh2.statechangehash).toEqual(Buffer.from([7, 8, 9]));
        // @ts-ignore
        expect(dh2.trieroot).toEqual(Buffer.from([4, 5, 6]));
    });

    it("should instantiate a transaction result", () => {
        const tr = new TxResult();
        expect(tr.clientTransaction).toBeUndefined();

        const tr2 = new TxResult({ clientTransaction: new ClientTransaction() });
        // @ts-ignore
        expect(tr2.clienttransaction).toBeDefined();
    });
});

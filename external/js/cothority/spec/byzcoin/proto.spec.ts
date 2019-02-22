import Long from "long";
import DataBody from "../../src/byzcoin/proto/data-body";
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
});

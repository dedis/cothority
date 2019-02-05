import Long from 'long';
import {
    CreateGenesisBlock, AddTxRequest
} from '../../src/byzcoin/proto';
import Darc from '../../src/darc/darc';

describe('ByzCoin Proto Tests', () => {
    it('should handle create genesis block messages', () => {
        const req = new CreateGenesisBlock({
            genesisDarc: new Darc(),
            blockInterval: Long.fromNumber(1),
            maxBlockSize: 42,
        });

        expect(req.genesisDarc).toBeDefined();
        expect(req.blockInterval.toNumber()).toBe(1);
        expect(req.maxBlockSize).toBe(42);

        expect(new CreateGenesisBlock()).toBeDefined();
    });

    it('should handle add tx request messages', () => {
        const req = new AddTxRequest({ skipchainID: Buffer.from([1, 2, 3]) });

        expect(req.skipchainID).toEqual(Buffer.from([1, 2, 3]));

        expect(new AddTxRequest()).toBeDefined();
    });
});

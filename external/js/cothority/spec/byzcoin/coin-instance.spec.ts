import Long from 'long';
import { ROSTER, startConodes, BLOCK_INTERVAL, SIGNER } from "../support/conondes";
import CoinInstance from '../../src/byzcoin/contracts/coin-instance';
import ByzCoinRPC from "../../src/byzcoin/byzcoin-rpc";
import Rules from "../../src/darc/rules";

describe('CoinInstance Tests', () => {
    const admin = SIGNER;
    const roster = ROSTER.slice(0, 4);

    beforeAll(async () => {
        await startConodes();
    });

    it('should spawn a coin instance', async () => {
        const darc = ByzCoinRPC.makeGenesisDarc([admin], roster);
        darc.addIdentity('spawn:coin', admin, Rules.OR);
        darc.addIdentity('invoke:coin.mint', admin, Rules.OR);
        darc.addIdentity('invoke:coin.transfer', admin, Rules.OR);

        const rpc = await ByzCoinRPC.newByzCoinRPC(roster, darc, BLOCK_INTERVAL);
        const ci = await CoinInstance.create(rpc, darc.baseID, [admin]);

        expect(ci.value.toNumber()).toBe(0);

        await ci.mint([admin], Long.fromNumber(1000));
        await ci.update();

        expect(ci.value.toNumber()).toBe(1000);

        const ci2 = await CoinInstance.create(rpc, darc.baseID, [admin, admin]);
        await ci.transfer(Long.fromNumber(50), ci2.id, [admin, admin]);

        await ci.update();
        await ci2.update();

        expect(ci.value.toNumber()).toBe(950);
        expect(ci2.value.toNumber()).toBe(50);
    });
});

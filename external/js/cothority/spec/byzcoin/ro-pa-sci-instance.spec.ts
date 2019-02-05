import Long from 'long';
import { startConodes, SIGNER, ROSTER, BLOCK_INTERVAL } from "../support/conondes";
import ByzCoinRPC from "../../src/byzcoin/byzcoin-rpc";
import Rules from "../../src/darc/rules";
import CoinInstance from "../../src/byzcoin/contracts/coin-instance";
import RoPaSciInstance, { RoPaSciStruct } from '../../src/byzcoin/contracts/ro-pa-sci-instance';
import { createHash } from 'crypto';
import ClientTransaction, { Instruction, Argument } from '../../src/byzcoin/client-transaction';
import Signer from '../../src/darc/signer';
import Darc from '../../src/darc/darc';

async function createInstance(rpc: ByzCoinRPC, stake: CoinInstance, darc: Darc, signer: Signer): Promise<RoPaSciInstance> {
    const fillup = Buffer.alloc(31);
    const fph = createHash("sha256");
    fph.update(Buffer.from([1]));
    fph.update(fillup);

    const rps = new RoPaSciStruct({
        description: 'test game',
        stake: stake.getCoin(),
        firstplayerhash: fph.digest(),
        firstplayer: -1,
        secondplayer: -1,
        secondplayeraccount: null,
    });

    const ctx = new ClientTransaction({
        instructions: [
            Instruction.createInvoke(
                stake.id,
                CoinInstance.contractID,
                "fetch",
                [new Argument({ name: "coins", value: Buffer.from(Long.fromNumber(100).toBytesLE()) })]
            ),
            Instruction.createSpawn(
                darc.baseID,
                RoPaSciInstance.contractID,
                [new Argument({ name: "struct", value: rps.toBytes() })],
            )
        ],
    });

    await ctx.updateCounters(rpc, [signer]);
    ctx.signWith([signer]);

    await rpc.sendTransactionAndWait(ctx);

    const iid = ctx.instructions[1].deriveId();
    const instance = await RoPaSciInstance.fromByzcoin(rpc, iid);
    instance.setChoice(1, fillup);

    return instance;
}

describe('Rock-Paper-Scisors Instance Tests', () => {
    const roster = ROSTER.slice(0, 4);

    beforeAll(async () => {
        await startConodes();
    });

    it('should play a game', async () => {
        const darc = ByzCoinRPC.makeGenesisDarc([SIGNER], roster);
        darc.addIdentity('spawn:coin', SIGNER, Rules.OR);
        darc.addIdentity('invoke:coin.mint', SIGNER, Rules.OR);
        darc.addIdentity('invoke:coin.fetch', SIGNER, Rules.OR);
        darc.addIdentity('spawn:ropasci', SIGNER, Rules.OR);

        const rpc = await ByzCoinRPC.newByzCoinRPC(roster, darc, BLOCK_INTERVAL);
        const p1 = await CoinInstance.create(rpc, darc.baseID, [SIGNER]);
        await p1.mint([SIGNER], Long.fromNumber(1000));
        await p1.update();
        const p2 = await CoinInstance.create(rpc, darc.baseID, [SIGNER, SIGNER]);
        await p2.mint([SIGNER], Long.fromNumber(1000));
        await p2.update();

        const rps = await createInstance(rpc, p1, darc, SIGNER);
        expect(rps).toBeDefined();

        await rps.second(p2, SIGNER, 2);

        await rps.confirm(p1);
        await rps.update();

        expect(rps.isDone()).toBeTruthy();
        expect(rps.adversaryID).toEqual(p2.id);
        expect(rps.adversaryChoice).toBe(2);
    }, 60 * 1000);
});

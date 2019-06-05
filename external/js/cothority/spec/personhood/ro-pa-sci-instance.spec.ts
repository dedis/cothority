import { createHash } from "crypto";
import Long from "long";
import ByzCoinRPC from "../../src/byzcoin/byzcoin-rpc";
import ClientTransaction, { Argument, Instruction } from "../../src/byzcoin/client-transaction";
import CoinInstance, { Coin } from "../../src/byzcoin/contracts/coin-instance";
import Darc from "../../src/darc/darc";
import { Rule } from "../../src/darc/rules";
import Signer from "../../src/darc/signer";
import Log from "../../src/log";
import RoPaSciInstance, { RoPaSciStruct } from "../../src/personhood/ro-pa-sci-instance";
import { BLOCK_INTERVAL, ROSTER, SIGNER, startConodes } from "../support/conondes";

async function createInstance(
    rpc: ByzCoinRPC,
    stake: CoinInstance,
    darc: Darc,
    signer: Signer,
): Promise<RoPaSciInstance> {
    const fillup = Buffer.alloc(31);
    const fph = createHash("sha256");
    fph.update(Buffer.from([1]));
    fph.update(fillup);

    const rps = new RoPaSciStruct({
        description: "test game",
        firstPlayer: -1,
        firstPlayerHash: fph.digest(),
        secondPlayer: -1,
        secondPlayerAccount: stake.id,
        stake: new Coin({name: stake.name, value: stake.value}),
    });

    const ctx = new ClientTransaction({
        instructions: [
            Instruction.createInvoke(
                stake.id,
                CoinInstance.contractID,
                CoinInstance.commandFetch,
                [new Argument({
                    name: CoinInstance.argumentCoins,
                    value: Buffer.from(Long.fromNumber(100).toBytesLE()),
                })],
            ),
            Instruction.createSpawn(
                darc.getBaseID(),
                RoPaSciInstance.contractID,
                [new Argument({ name: "struct", value: rps.toBytes() })],
            ),
        ],
    });

    await ctx.updateCountersAndSign(rpc, [[signer], [signer]]);

    await rpc.sendTransactionAndWait(ctx);

    const iid = ctx.instructions[1].deriveId();
    const instance = await RoPaSciInstance.fromByzcoin(rpc, iid);
    instance.setChoice(1, fillup);

    return instance;
}

describe("Rock-Paper-scissors Instance Tests", () => {
    const roster = ROSTER.slice(0, 4);

    beforeAll(async () => {
        await startConodes();
    });

    it("should play a game", async () => {
        const coinType = CoinInstance.coinIID(Buffer.from("secure coin"));
        const darc = ByzCoinRPC.makeGenesisDarc([SIGNER], roster);
        darc.addIdentity("spawn:coin", SIGNER, Rule.OR);
        darc.addIdentity("invoke:coin.mint", SIGNER, Rule.OR);
        darc.addIdentity("invoke:coin.fetch", SIGNER, Rule.OR);
        darc.addIdentity("spawn:ropasci", SIGNER, Rule.OR);

        const rpc = await ByzCoinRPC.newByzCoinRPC(roster, darc, BLOCK_INTERVAL);
        const p1 = await CoinInstance.spawn(rpc, darc.getBaseID(), [SIGNER], coinType);
        await p1.mint([SIGNER], Long.fromNumber(1000));
        await p1.update();
        const p2 = await CoinInstance.spawn(rpc, darc.getBaseID(), [SIGNER], coinType);
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

import Long from "long";
import ClientTransaction, { Argument, Instruction } from "../../src/byzcoin/client-transaction";
import DarcInstance from "../../src/byzcoin/contracts/darc-instance";
import { IIdentity } from "../../src/darc";
import { SIGNER } from "../support/conondes";

const updater = new class {
    getSignerCounters(signers: IIdentity[], increment: number): Promise<Long[]> {
        return Promise.resolve(signers.map(() => Long.fromNumber(increment)));
    }
    updateCachedCounters(signers: IIdentity[]): Promise<Long[]> {
        return this.getSignerCounters(signers, 1);
    }
    getNextCounter(signer: IIdentity): Long {
        return Long.fromNumber(1);
    }

}();

describe("ClientTransaction Tests", () => {
    const IID = Buffer.allocUnsafe(32);

    it("should create a spawn instruction", async () => {
        const instr = Instruction.createSpawn(IID, "abc", []);
        await instr.updateCounters(updater, [SIGNER]);
        instr.signWith(instr.hash(), [SIGNER]);

        expect(instr.signerCounter[0].toNumber()).toBe(1);
        expect(instr.instanceID).toEqual(IID);
        expect(instr.signatures.length).toBe(1);
        expect(instr.type).toBe(Instruction.typeSpawn);
        expect(instr.deriveId).toBeDefined();
    });

    it("should create an invoke instruction", async () => {
        const args = [new Argument({ name: "a", value: Buffer.from("b") })];
        const instr = Instruction.createInvoke(IID, "abc", DarcInstance.commandEvolve, args);
        await instr.updateCounters(updater, [SIGNER]);
        instr.signWith(instr.hash(), [SIGNER]);

        expect(instr.signerCounter[0].toNumber()).toBe(1);
        expect(instr.instanceID).toEqual(IID);
        expect(instr.signatures.length).toBe(1);
        expect(instr.type).toBe(Instruction.typeInvoke);
        expect(instr.deriveId()).toBeDefined();
    });

    it("should create a delete instruction", async () => {
        const instr = Instruction.createDelete(IID, "abc");
        await instr.updateCounters(updater, [SIGNER]);
        instr.signWith(instr.hash(), [SIGNER]);

        expect(instr.signerCounter[0].toNumber()).toBe(1);
        expect(instr.instanceID).toEqual(IID);
        expect(instr.signatures.length).toBe(1);
        expect(instr.type).toBe(Instruction.typeDelete);
        expect(instr.deriveId()).toBeDefined();
    });

    it("should create a transaction", async () => {
        const ctx = new ClientTransaction({
            instructions: [
                Instruction.createDelete(IID, "abc"),
                Instruction.createDelete(IID, "def"),
            ],
        });
        await ctx.updateCountersAndSign(updater, [[SIGNER], [SIGNER]]);

        expect(ctx.hash()).toBeDefined();

        // update empty instruction list
        const ctx2 = new ClientTransaction({ instructions: [] });
        await ctx2.updateCounters(updater, []);
    });
});

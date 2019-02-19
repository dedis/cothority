import Long from "long";
import ClientTransaction, { Argument, Instruction } from "../../src/byzcoin/client-transaction";
import { IIdentity } from "../../src/darc/identity-wrapper";
import { SIGNER } from "../support/conondes";

const updater = new class {
    getSignerCounters(signers: IIdentity[], increment: number): Promise<Long[]> {
        return Promise.resolve(signers.map(() => Long.fromNumber(increment)));
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
        expect(instr.type).toBe(0);
        expect(instr.deriveId).toBeDefined();
    });

    it("should create an invoke instruction", async () => {
        const args = [new Argument({ name: "a", value: Buffer.from("b") })];
        const instr = Instruction.createInvoke(IID, "abc", "evolve", args);
        await instr.updateCounters(updater, [SIGNER]);
        instr.signWith(instr.hash(), [SIGNER]);

        expect(instr.signerCounter[0].toNumber()).toBe(1);
        expect(instr.instanceID).toEqual(IID);
        expect(instr.signatures.length).toBe(1);
        expect(instr.type).toBe(1);
        expect(instr.deriveId()).toBeDefined();
    });

    it("should create a delete instruction", async () => {
        const instr = Instruction.createDelete(IID, "abc");
        await instr.updateCounters(updater, [SIGNER]);
        instr.signWith(instr.hash(), [SIGNER]);

        expect(instr.signerCounter[0].toNumber()).toBe(1);
        expect(instr.instanceID).toEqual(IID);
        expect(instr.signatures.length).toBe(1);
        expect(instr.type).toBe(2);
        expect(instr.deriveId()).toBeDefined();
    });

    it("should create a transaction", async () => {
        const ctx = new ClientTransaction({
            instructions: [
                Instruction.createDelete(IID, "abc"),
                Instruction.createDelete(IID, "def"),
            ],
        });
        await ctx.updateCounters(updater, [SIGNER]);
        ctx.signWith([SIGNER]);

        expect(ctx.hash()).toBeDefined();

        // update empty instruction list
        const ctx2 = new ClientTransaction({ instructions: [] });
        await ctx2.updateCounters(updater, [SIGNER]);
    });
});

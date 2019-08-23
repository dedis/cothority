import ByzCoinRPC from "../../src/byzcoin/byzcoin-rpc";
import ClientTransaction, { Argument, Instruction } from "../../src/byzcoin/client-transaction";
import Darc from "../../src/darc/darc";
import { Rule } from "../../src/darc/rules";
import Signer from "../../src/darc/signer";
import CredentialsInstance, { CredentialStruct } from "../../src/personhood/credentials-instance";
import { BLOCK_INTERVAL, ROSTER, SIGNER, startConodes } from "../support/conondes";

async function createInstance(rpc: ByzCoinRPC, signers: Signer[], darc: Darc, cred: CredentialStruct):
    Promise<CredentialsInstance> {
    const ctx = ClientTransaction.make(rpc.getProtocolVersion(), Instruction.createSpawn(
        darc.getBaseID(),
        CredentialsInstance.contractID,
        [
            new Argument({ name: CredentialsInstance.argumentDarcID, value: darc.getBaseID() }),
            new Argument({ name: CredentialsInstance.argumentCredential, value: cred.toBytes() }),
        ],
    ));
    await ctx.updateCounters(rpc, [signers]);
    ctx.signWith([signers]);

    await rpc.sendTransactionAndWait(ctx);

    return CredentialsInstance.fromByzcoin(rpc, ctx.instructions[0].deriveId());
}

describe("CredentialsInstance Tests", () => {
    const roster = ROSTER.slice(0, 4);

    beforeAll(async () => {
        await startConodes();
    });

    it("should create a credential instance", async () => {
        const darc = ByzCoinRPC.makeGenesisDarc([SIGNER], roster);
        darc.addIdentity("spawn:credential", SIGNER, Rule.OR);
        darc.addIdentity("invoke:credential.update", SIGNER, Rule.OR);

        const rpc = await ByzCoinRPC.newByzCoinRPC(roster, darc, BLOCK_INTERVAL);

        const cred = new CredentialStruct();
        const ci = await createInstance(rpc, [SIGNER], darc, cred);
        expect(ci).toBeDefined();
        expect(ci.darcID).toEqual(darc.getBaseID());

        // set non-existing credential
        await ci.setAttribute("personhood", "ed25519", SIGNER.toBytes());
        await ci.sendUpdate([SIGNER]);
        await ci.update();
        expect(ci.getAttribute("personhood", "ed25519")).toEqual(SIGNER.toBytes());

        // set a different credential
        await ci.setAttribute("personhood", "abc", Buffer.from("abc"));
        await ci.sendUpdate([SIGNER]);
        await ci.update();
        expect(ci.getAttribute("personhood", "ed25519")).toEqual(SIGNER.toBytes());
        expect(ci.getAttribute("personhood", "abc")).toEqual(Buffer.from("abc"));

        // update a credential
        await ci.setAttribute("personhood", "abc", Buffer.from("def"));
        await ci.sendUpdate([SIGNER]);
        await ci.update();
        expect(ci.getAttribute("personhood", "ed25519")).toEqual(SIGNER.toBytes());
        expect(ci.getAttribute("personhood", "abc")).toEqual(Buffer.from("def"));

        expect(ci.getAttribute("personhood", "a")).toBeUndefined();
        expect(ci.getAttribute("a", "")).toBeUndefined();
    });

    it("allow to set the credential", async () => {
        const cs = new CredentialStruct();
        cs.setAttribute("one", "two", Buffer.from("three"));
        cs.setAttribute("one", "four", undefined);
        cs.setAttribute("one", "five", Buffer.from("six"));
        const cred = cs.copy().getCredential("one");
        expect(cred.attributes.length).toBe(3);
        expect(cs.deleteAttribute("one", "seven")).toBeUndefined();
        expect(cs.deleteAttribute("one", "four")).toEqual(Buffer.alloc(0));
        expect(cs.deleteAttribute("one", "five")).toEqual(Buffer.from("six"));
        expect(cs.getCredential("one").attributes.length).toBe(1);
        cs.setCredential("one", cred);
        expect(cs.getCredential("one").attributes.length).toBe(3);
    });
});

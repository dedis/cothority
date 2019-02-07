import ByzCoinRPC from "../../src/byzcoin/byzcoin-rpc";
import ClientTransaction, { Argument, Instruction } from "../../src/byzcoin/client-transaction";
import CredentialsInstance, { CredentialStruct } from "../../src/byzcoin/contracts/credentials-instance";
import Darc from "../../src/darc/darc";
import Rules from "../../src/darc/rules";
import Signer from "../../src/darc/signer";
import { BLOCK_INTERVAL, ROSTER, SIGNER, startConodes } from "../support/conondes";

async function createInstance(rpc: ByzCoinRPC, signers: Signer[], darc: Darc, cred: CredentialStruct) {
    const ctx = new ClientTransaction({
        instructions: [
            Instruction.createSpawn(
                darc.baseID,
                CredentialsInstance.contractID,
                [
                    new Argument({ name: "darcID", value: darc.baseID }),
                    new Argument({ name: "credential", value: cred.toBytes() }),
                ],
            ),
        ],
    });
    await ctx.updateCounters(rpc, signers);
    ctx.signWith(signers);

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
        darc.addIdentity("spawn:credential", SIGNER, Rules.OR);
        darc.addIdentity("invoke:credential.update", SIGNER, Rules.OR);

        const rpc = await ByzCoinRPC.newByzCoinRPC(roster, darc, BLOCK_INTERVAL);

        const cred = new CredentialStruct();
        const ci = await createInstance(rpc, [SIGNER], darc, cred);
        expect(ci).toBeDefined();
        expect(ci.darcID).toEqual(darc.baseID);

        // set non-existing credential
        await ci.setAttribute(SIGNER, "personhood", "ed25519", SIGNER.toBytes());
        await ci.update();
        expect(ci.getAttribute("personhood", "ed25519")).toEqual(SIGNER.toBytes());

        // set a different credential
        await ci.setAttribute(SIGNER, "personhood", "abc", Buffer.from("abc"));
        await ci.update();
        expect(ci.getAttribute("personhood", "ed25519")).toEqual(SIGNER.toBytes());
        expect(ci.getAttribute("personhood", "abc")).toEqual(Buffer.from("abc"));

        // update a credential
        await ci.setAttribute(SIGNER, "personhood", "abc", Buffer.from("def"));
        await ci.update();
        expect(ci.getAttribute("personhood", "ed25519")).toEqual(SIGNER.toBytes());
        expect(ci.getAttribute("personhood", "abc")).toEqual(Buffer.from("def"));

        expect(ci.getAttribute("personhood", "a")).toBeNull();
        expect(ci.getAttribute("a", "")).toBeNull();
    });
});

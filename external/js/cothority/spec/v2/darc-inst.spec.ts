import { elementAt } from "rxjs/operators";

import { DarcInstance } from "../../src/byzcoin/contracts";
import { Darc, SignerEd25519 } from "../../src/darc";
import { DarcContract, DarcInst } from "../../src/v2/byzcoin/contracts";

import { BCTest } from "../helpers/bctest";
import { SIGNER } from "../support/conondes";
import { HistoryObs } from "../support/historyObs";

describe("DarcInst should", () => {
    it("retrieve an instance from byzcoin", async () => {
        const {genesis, rpc} = await BCTest.singleton();
        const dbs = await DarcInst.retrieve(rpc, genesis.getBaseID());
        expect(dbs.getValue().inst.id.equals(genesis.getBaseID())).toBeTruthy();
    });

    it("update when the darc is updated", async () => {
        const {genesis, rpc, tx} = await BCTest.singleton();
        const d = Darc.createBasic([SIGNER], [SIGNER], Buffer.from("new darc"));
        await DarcInstance.spawn(rpc, genesis.getBaseID(), [SIGNER], d);
        const dbs = await DarcInst.retrieve(rpc, d.getBaseID());
        expect(dbs.getValue().inst.id.equals(d.getBaseID())).toBeTruthy();

        const newDarc = dbs.pipe(elementAt(1)).toPromise();
        dbs.setDescription(tx, Buffer.from("new description"));
        await tx.send([[SIGNER]], 10);

        expect((await newDarc).description).toEqual(Buffer.from("new description"));
    });

    it("update rules", async () => {
        const {genesis, rpc, tx} = await BCTest.singleton();
        const newDarc = Darc.createBasic([SIGNER], [SIGNER], Buffer.from("darc1"));
        await DarcInstance.spawn(rpc, genesis.getBaseID(), [SIGNER], newDarc);
        const dbs = await DarcInst.retrieve(rpc, newDarc.getBaseID());
        const hist = new HistoryObs();

        // Create updates with the description:#signers:#evolvers
        dbs.subscribe((d) => {
            const signLen = d.rules.getRule(DarcContract.ruleSign).getIdentities().length;
            const evolveLen = d.rules.getRule(DarcContract.ruleEvolve).getIdentities().length;
            hist.push(`${d.description.toString()}:${signLen}:${evolveLen}`);
        });
        await hist.resolve("darc1:1:1");

        dbs.setDescription(tx, Buffer.from("darc2"));
        await tx.send([[SIGNER]]);
        await hist.resolve("darc2:1:1");

        // Change the evolver and use it to evolve future darcs
        const newEvolver = SignerEd25519.random();
        dbs.addToRules(tx, [DarcContract.ruleEvolve, newEvolver]);
        await tx.send([[SIGNER]], 10);
        await hist.resolve("darc2:1:2");

        // Add both signer and evolver
        const newSigner = SignerEd25519.random();
        const newEvolver2 = SignerEd25519.random();
        dbs.addToRules(tx, [DarcContract.ruleSign, newSigner], [DarcContract.ruleEvolve, newEvolver2]);
        await tx.send([[newEvolver]], 10);
        await hist.resolve("darc2:2:3");

        // Remove the old evolver
        dbs.rmFromRules(tx, [DarcContract.ruleEvolve, newEvolver]);
        await tx.send([[newEvolver2]], 10);
        await hist.resolve("darc2:2:2");

        // Reset the evolver
        dbs.setRules(tx, [DarcContract.ruleSign, newSigner], [DarcContract.ruleEvolve, newEvolver2]);
        await tx.send([[newEvolver2]], 10);
        await hist.resolve("darc2:1:1");

        // Reset the signer (first add, then set)
        dbs.addToRules(tx, [DarcContract.ruleSign, newSigner]);
        await tx.send([[newEvolver2]], 10);
        await hist.resolve("darc2:2:1");

        dbs.setRules(tx, [DarcContract.ruleSign, newSigner]);
        await tx.send([[newEvolver2]], 10);
        await hist.resolve("darc2:1:1");
        expect(dbs.getValue().rules.getRule(DarcContract.ruleEvolve).getIdentities()[0]).toBe(newEvolver2.toString());
    });
});

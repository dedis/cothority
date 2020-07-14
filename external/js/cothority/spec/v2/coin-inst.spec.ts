import Long from "long";

import { Log } from "../../src";
import { CoinContract, CoinInst } from "../../src/v2/byzcoin/contracts";
import { BCTest } from "../helpers/bctest";
import { SIGNER } from "../support/conondes";
import { HistoryObs } from "../support/historyObs";

describe("CoinInst should", () => {
    const name = Buffer.alloc(32);

    beforeAll(async () => {
        name.write("coinName");
    });

    it("retrieve an instance from byzcoin", async () => {
        const {genesisInst, tx, rpc} = await BCTest.singleton();
        const coinID = genesisInst.spawnCoin(tx, name);
        await tx.send([[SIGNER]], 10);
        const ci = await CoinInst.retrieve(rpc, coinID);
        expect(ci.getValue().name.equals(name)).toBeTruthy();
    });

    it("mint some coins", async () => {
        const {genesisInst, tx, rpc} = await BCTest.singleton();
        const coinID = genesisInst.spawnCoin(tx, name);
        await tx.send([[SIGNER]], 10);

        const ci = await CoinInst.retrieve(rpc, coinID);
        const h = new HistoryObs();
        ci.subscribe((c) => h.push(c.value.toString()));
        await h.resolve("0");

        ci.mint(tx, Long.fromNumber(1e6));
        await tx.send([[SIGNER]], 10);
        await h.resolve(1e6.toString());
    });

    it("transfer coins", async () => {
        const {genesisInst, tx, rpc} = await BCTest.singleton();

        Log.lvl2("Spawning 2 coins");
        const sourceID = genesisInst.spawnCoin(tx, name);
        const targetID = genesisInst.spawnCoin(tx, name);
        CoinContract.mint(tx, sourceID, Long.fromNumber(1e6));
        CoinContract.transfer(tx, sourceID, targetID, Long.fromNumber(1e5));
        await tx.send([[SIGNER]], 10);

        Log.lvl2("Getting coins and values");
        const target = await CoinInst.retrieve(rpc, targetID);
        const hTarget = new HistoryObs();
        target.subscribe((ci) => hTarget.push(ci.value.toString()));
        await hTarget.resolve(1e5.toString());

        Log.lvl2("Transferring some coins from source to target");
        const source = await CoinInst.retrieve(rpc, sourceID);
        const hSource = new HistoryObs();
        source.subscribe((ci) => hSource.push(ci.value.toString()));
        source.mint(tx, Long.fromNumber(1e6));
        source.transfer(tx, targetID, Long.fromNumber(2e5));
        await tx.send([[SIGNER]], 10);
        await hSource.resolve(9e5.toString(), 17e5.toString());
        await hTarget.resolve(3e5.toString());

        Log.lvl2("Using fetch and store for transfer");
        source.fetch(tx, Long.fromNumber(3e5));
        target.store(tx);
        await tx.send([[SIGNER]], 10);
        await hSource.resolve(14e5.toString());
        await hTarget.resolve(6e5.toString());
    }, 600000);
});

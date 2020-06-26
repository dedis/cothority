import fs from "fs";
import Long from "long";

import Log from "../../src/log";

import ByzCoinRPC from "../../src/byzcoin/byzcoin-rpc";
import SignerEd25519 from "../../src/darc/signer-ed25519";
import { Roster } from "../../src/network";

import { BEvmClient, BEvmService, EvmAccount, EvmContract } from "../../src/bevm";

import { ROSTER, startConodes } from "../support/conondes";

describe("BEvmClient", async () => {
    let roster: Roster;
    let rosterTOML: string;
    let byzcoinRPC: ByzCoinRPC;
    let admin: SignerEd25519;
    let srv: BEvmService;
    let client: BEvmClient;

    /* tslint:disable:max-line-length */
    const candyBytecode = Buffer.from(`
608060405234801561001057600080fd5b506040516020806101cb833981018060405281019080805190602001909291905050508060008190555080600181905550600060028190555050610172806100596000396000f30060806040526004361061004c576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff168063a1ff2f5214610051578063ea319f281461007e575b600080fd5b34801561005d57600080fd5b5061007c600480360381019080803590602001909291905050506100a9565b005b34801561008a57600080fd5b5061009361013c565b6040518082815260200191505060405180910390f35b6001548111151515610123576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260058152602001807f6572726f7200000000000000000000000000000000000000000000000000000081525060200191505060405180910390fd5b8060015403600181905550806002540160028190555050565b60006001549050905600a165627a7a723058207721a45f17c0e0f57e255f33575281d17f1a90d3d58b51688230d93c460a19aa0029
`.trim(), "hex");
    const candyAbi = `
[{"constant":false,"inputs":[{"name":"candies","type":"uint256"}],"name":"eatCandy","outputs":[],"payable":false,"stateMutability":"nonpayable","type":"function"},{"constant":true,"inputs":[],"name":"getRemainingCandies","outputs":[{"name":"","type":"uint256"}],"payable":false,"stateMutability":"view","type":"function"},{"inputs":[{"name":"_candies","type":"uint256"}],"payable":false,"stateMutability":"nonpayable","type":"constructor"}]
`.trim();
    /* tslint:enable:max-line-length */
    const WEI_PER_ETHER = Long.fromString("1000000000000000000");

    beforeAll(async () => {
        await startConodes();

        roster = ROSTER.slice(0, 4);
        rosterTOML = fs.readFileSync(process.cwd() + "/spec/support/public.toml").toString();

        const srvConode = roster.list[0];
        srv = new BEvmService(srvConode);
        srv.setTimeout(1000);

        admin = SignerEd25519.random();

        const darc = ByzCoinRPC.makeGenesisDarc(
            [admin], roster, "genesis darc");
        [
            "spawn:bevm",
            "delete:bevm",
            "invoke:bevm.credit",
            "invoke:bevm.transaction",
        ].forEach((rule) => {
            darc.rules.appendToRule(rule, admin, "|");
        });

        byzcoinRPC = await ByzCoinRPC.newByzCoinRPC(roster, darc, Long.fromNumber(1e9));

        client = await BEvmClient.spawn(byzcoinRPC, darc.getBaseID(), [admin]);
        client.setBEvmService(srv);

    }, 30 * 1000);

    it("should correctly sign a hash", () => {
        const hash = Buffer.from("c289e67875d147429d2ffc5cc58e9a1486d581bef5aeca63017ad7855f8dab26", "hex");
        /* tslint:disable:max-line-length */
        const expectedSig = Buffer.from("e6efff1077fe39f6a8b3e9dca6f2462d2d32aa51e911a7d7abd8741da6b09b9e35c184ec193af4258bc603cd20b48ceb2ff9317742741e9c4e8e97dfe1d6d39d01", "hex");
        /* tslint:enable:max-line-length */

        const privKey = Buffer.from("c87509a1c067bbde78beb793e6fa76530b6382a4c0241e5e4a9ec0a0f44dc0d3", "hex");
        const account = new EvmAccount("test", privKey);

        const sig = account.sign(hash);

        expect(sig).toEqual(expectedSig);
    });

    it("should successfully deploy and interact with a contract", async () => {
        const privKey = Buffer.from("c87509a1c067bbde78beb793e6fa76530b6382a4c0241e5e4a9ec0a0f44dc0d3", "hex");
        const expectedAccountAddress = Buffer.from("627306090abab3a6e1400e9345bc60c78a8bef57", "hex");
        const expectedContractAddress = Buffer.from("8cdaf0cd259887258bc13a92c0a6da92698644c0", "hex");

        const account = new EvmAccount("test", privKey);
        expect(account.address).toEqual(expectedAccountAddress);

        const contract = new EvmContract("Candy", candyBytecode, candyAbi);

        const amount = Buffer.from(WEI_PER_ETHER.mul(5).toBytesBE());

        Log.lvl2("Credit an account with:", amount);
        await expectAsync(client.creditAccount([admin], account.address, amount)).toBeResolved();

        Log.lvl2("Deploy a Candy contract");
        await expectAsync(client.deploy([admin],
                                        1e7,
                                        1,
                                        0,
                                        account,
                                        contract,
                                        [JSON.stringify(String(100))],
                                       )).toBeResolved();
        expect(contract.addresses[0]).toEqual(expectedContractAddress);

        for (let nbCandies = 1; nbCandies <= 10; nbCandies++) {
            Log.lvl2(`Eat ${nbCandies} candies`);
            await expectAsync(client.transaction([admin],
                                                 1e7,
                                                 1,
                                                 0,
                                                 account,
                                                 contract,
                                                 0,
                                                 "eatCandy",
                                                 [JSON.stringify(String(nbCandies))],
                                                )).toBeResolved();
        }

        Log.lvl2("Retrieve number of remaining candies");
        const expectedRemainingCoins = 100 - (10 * 11 / 2);
        await expectAsync(client.call(byzcoinRPC.genesisID,
                                      rosterTOML,
                                      // roster.toTOML(),
                                      client.id,
                                      account,
                                      contract,
                                      0,
                                      "getRemainingCandies",
                                      [],
                                     )).toBeResolvedTo(expectedRemainingCoins);
    }, 60000); // Extend Jasmine default timeout interval to 1 minute
});

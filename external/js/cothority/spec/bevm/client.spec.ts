import BN from "bn.js";
import fs from "fs";
import Long from "long";

// For debugging
// import Log from "../../src/log";

import ByzCoinRPC from "../../src/byzcoin/byzcoin-rpc";
import SignerEd25519 from "../../src/darc/signer-ed25519";
import { Roster } from "../../src/network";

import { BEvmClient, BEvmService, EvmAccount, EvmContract,
    WEI_PER_ETHER } from "../../src/bevm";

import { ROSTER, startConodes } from "../support/conondes";

describe("BEvmClient", async () => {
    let roster: Roster;
    let rosterTOML: string;
    let byzcoinRPC: ByzCoinRPC;
    let admin: SignerEd25519;
    let srv: BEvmService;
    let client: BEvmClient;

    const candyBytecode = Buffer.from(`
608060405234801561001057600080fd5b506040516020806101cb833981018060405281019080\
805190602001909291905050508060008190555080600181905550600060028190555050610172\
806100596000396000f30060806040526004361061004c576000357c0100000000000000000000\
000000000000000000000000000000000000900463ffffffff168063a1ff2f5214610051578063\
ea319f281461007e575b600080fd5b34801561005d57600080fd5b5061007c6004803603810190\
80803590602001909291905050506100a9565b005b34801561008a57600080fd5b506100936101\
3c565b6040518082815260200191505060405180910390f35b6001548111151515610123576040\
517f08c379a0000000000000000000000000000000000000000000000000000000008152600401\
8080602001828103825260058152602001807f6572726f72000000000000000000000000000000\
00000000000000000000000081525060200191505060405180910390fd5b806001540360018190\
5550806002540160028190555050565b60006001549050905600a165627a7a723058207721a45f\
17c0e0f57e255f33575281d17f1a90d3d58b51688230d93c460a19aa0029
`.trim(), "hex");
    const candyAbi = `
[{"constant":false,"inputs":[{"name":"candies","type":"uint256"}],"name":"eatC\
andy","outputs":[],"payable":false,"stateMutability":"nonpayable","type":"func\
tion"},{"constant":true,"inputs":[],"name":"getRemainingCandies","outputs":[{"\
name":"","type":"uint256"}],"payable":false,"stateMutability":"view","type":"f\
unction"},{"inputs":[{"name":"_candies","type":"uint256"}],"payable":false,"st\
ateMutability":"nonpayable","type":"constructor"}]
`.trim();

    beforeAll(async () => {
        await startConodes();

        roster = ROSTER.slice(0, 4);
        // Taken from  .../spec/support/conondes.ts
        rosterTOML = fs.readFileSync(
            process.cwd() + "/spec/support/public.toml").toString();

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

        byzcoinRPC = await ByzCoinRPC.newByzCoinRPC(roster, darc,
                                                    Long.fromNumber(1e9));

        client = await BEvmClient.spawn(byzcoinRPC, darc.getBaseID(), [admin]);
        client.setBEvmService(srv);

    }, 30 * 1000);

    it("should successfully deploy and interact with a contract", async () => {
        const privKey = Buffer.from(
            "c87509a1c067bbde78beb793e6fa76530b6382a4c0241e5e4a9ec0a0f44dc0d3",
            "hex");
        const expectedContractAddress = Buffer.from(
            "8cdaf0cd259887258bc13a92c0a6da92698644c0", "hex");

        const account = new EvmAccount("test", privKey);
        const contract = new EvmContract("Candy", candyBytecode, candyAbi);

        const amount = WEI_PER_ETHER.mul(5);

        // Credit an account so that we can perform actions
        await expectAsync(client.creditAccount(
            [admin],
            account,
            amount,
        )).toBeResolved();

        // Deploy a Candy contract with 100 candies
        await expectAsync(client.deploy(
            [admin],
            1e7,
            1,
            0,
            account,
            contract,
            [100],
        )).toBeResolved();
        expect(contract.addresses[0]).toEqual(expectedContractAddress);

        // Eat a bunch of candies
        for (let nbCandies = 1; nbCandies <= 10; nbCandies++) {
            await expectAsync(client.transaction(
                [admin],
                1e7,
                1,
                0,
                account,
                contract,
                0,
                "eatCandy",
                [nbCandies],
            )).toBeResolved();
        }

        // Retrieve number of remaining candies, which should be 100 - (1 + 2 +
        // ... + 10) = 100 - (10 * 11 / 2)
        const expectedRemainingCandies = new BN(100 - (10 * 11 / 2));
        const [remainingCandies] = await client.call(
            byzcoinRPC.genesisID,
            rosterTOML,
            client.id,
            account,
            contract,
            0,
            "getRemainingCandies",
        );

        expect(remainingCandies.eq(expectedRemainingCandies)).toBe(true);
    }, 60000); // Extend Jasmine default timeout interval to 1 minute

    it("should successfully handle large numbers", async () => {
        const privKey = Buffer.from(
            "d87509a1c067bbde78beb793e6fa76530b6382a4c0241e5e4a9ec0a0f44dc0d3",
            "hex");
        const account = new EvmAccount("test", privKey);
        const contract = new EvmContract("Candy", candyBytecode, candyAbi);

        // Credit an account so that we can perform actions
        await expectAsync(client.creditAccount(
            [admin],
            account,
            WEI_PER_ETHER.mul(5),
        )).toBeResolved();

        // 2^128
        const nbCandies = new BN("340282366920938463463374607431768211456");

        // Deploy a Candy contract
        await expectAsync(client.deploy(
            [admin],
            1e7,
            1,
            0,
            account,
            contract,
            [nbCandies],
        )).toBeResolved();

        // Retrieve number of remaining candies
        const [remainingCandies] = await client.call(
            byzcoinRPC.genesisID,
            rosterTOML,
            client.id,
            account,
            contract,
            0,
            "getRemainingCandies",
        );

        expect(remainingCandies.eq(nbCandies)).toBe(true);
    }, 60000); // Extend Jasmine default timeout interval to 1 minute
});

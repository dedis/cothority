import Long from "long";

import { BigNumber } from "@ethersproject/bignumber";

// For debugging
// import Log from "../../src/log";

import ByzCoinRPC from "../../src/byzcoin/byzcoin-rpc";
import SignerEd25519 from "../../src/darc/signer-ed25519";
import { Roster } from "../../src/network";

import { BEvmInstance, EvmAccount, EvmContract,
    WEI_PER_ETHER } from "../../src/bevm";

import { ROSTER, startConodes } from "../support/conondes";

describe("BEvmInstance", async () => {
    let roster: Roster;
    let byzcoinRPC: ByzCoinRPC;
    let admin: SignerEd25519;
    let bevmInstance: BEvmInstance;

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

        bevmInstance = await BEvmInstance.spawn(byzcoinRPC, darc.getBaseID(), [admin]);
    }, 30 * 1000);

    it("should successfully deploy and interact with a contract", async () => {
        // NOTE: Make sure the account privKey is different for each test, to
        // avoid issues with nonces
        const privKey = Buffer.from(
            "c87509a1c067bbde78beb793e6fa76530b6382a4c0241e5e4a9ec0a0f44dc0d3",
            "hex");
        const expectedContractAddress = Buffer.from(
            "8cdaf0cd259887258bc13a92c0a6da92698644c0", "hex");

        const account = new EvmAccount("test", privKey);
        const contract = new EvmContract("Candy", candyBytecode, candyAbi);

        const amount = WEI_PER_ETHER.mul(5);

        // Credit an account so that we can perform actions
        await expectAsync(bevmInstance.creditAccount(
            [admin],
            account,
            amount,
        )).toBeResolved();

        // Deploy a Candy contract with 100 candies
        await expectAsync(bevmInstance.deploy(
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
            await expectAsync(bevmInstance.transaction(
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
        const expectedRemainingCandies = BigNumber.from(100 - (10 * 11 / 2));
        const [remainingCandies] = await bevmInstance.call(
            account,
            contract,
            0,
            "getRemainingCandies",
        );

        expect(remainingCandies.eq(expectedRemainingCandies)).toBe(true);
    }, 60000); // Extend Jasmine default timeout interval to 1 minute

    it("should successfully handle large numbers", async () => {
        // NOTE: Make sure the account privKey is different for each test, to
        // avoid issues with nonces
        const privKey = Buffer.from(
            "d87509a1c067bbde78beb793e6fa76530b6382a4c0241e5e4a9ec0a0f44dc0d3",
            "hex");
        const account = new EvmAccount("test", privKey);
        const contract = new EvmContract("Candy", candyBytecode, candyAbi);

        // Credit an account so that we can perform actions
        await expectAsync(bevmInstance.creditAccount(
            [admin],
            account,
            WEI_PER_ETHER.mul(5),
        )).toBeResolved();

        // 2^128
        const nbCandies = BigNumber.from("340282366920938463463374607431768211456");

        // Deploy a Candy contract
        await expectAsync(bevmInstance.deploy(
            [admin],
            1e7,
            1,
            0,
            account,
            contract,
            [nbCandies],
        )).toBeResolved();

        // Retrieve number of remaining candies
        const [remainingCandies] = await bevmInstance.call(
            account,
            contract,
            0,
            "getRemainingCandies",
        );

        expect(remainingCandies.eq(nbCandies)).toBe(true);
    }, 60000); // Extend Jasmine default timeout interval to 1 minute

    it("should handle ABIv2", async () => {
        // Test code:
        //
        // pragma experimental ABIEncoderV2;
        // pragma solidity ^0.5.0;
        //
        // contract ABIv2 {
        //     struct S {
        //         uint256 v1;
        //         uint256 v2;
        //     }
        //
        //     function squares(uint256 limit) public view returns (S[] memory) {
        //         S[] memory result = new S[](limit);
        //
        //         for (uint256 i = 0; i < limit; i++) {
        //             S memory s = S(i, i * i);
        //             result[i] = s;
        //         }
        //
        //         return result;
        //     }
        // }

        const abiV2Bytecode = Buffer.from(`
608060405234801561001057600080fd5b506102e4806100206000396000f3fe60806040523480\
1561001057600080fd5b506004361061002b5760003560e01c80631d1d15d414610030575b6000\
80fd5b61004a60048036036100459190810190610148565b610060565b60405161005791906102\
25565b60405180910390f35b606080826040519080825280602002602001820160405280156100\
9d57816020015b61008a6100ff565b8152602001906001900390816100825790505b5090506000\
8090505b838110156100f5576100b6610119565b60405180604001604052808381526020018384\
028152509050808383815181106100dc57fe5b6020026020010181905250508080600101915050\
6100a6565b5080915050919050565b604051806040016040528060008152602001600081525090\
565b604051806040016040528060008152602001600081525090565b6000813590506101428161\
028a565b92915050565b60006020828403121561015a57600080fd5b6000610168848285016101\
33565b91505092915050565b600061017d83836101e7565b60408301905092915050565b600061\
019482610257565b61019e818561026f565b93506101a983610247565b8060005b838110156101\
da5781516101c18882610171565b97506101cc83610262565b9250506001810190506101ad565b\
5085935050505092915050565b6040820160008201516101fd6000850182610216565b50602082\
01516102106020850182610216565b50505050565b61021f81610280565b82525050565b600060\
2082019050818103600083015261023f8184610189565b905092915050565b6000819050602082\
019050919050565b600081519050919050565b6000602082019050919050565b60008282526020\
8201905092915050565b6000819050919050565b61029381610280565b811461029e57600080fd\
5b5056fea365627a7a72315820381504cadc97d1a39d5aeddd0d1fb4dab2f25e091ddab0f5e5c3\
edecac2207db6c6578706572696d656e74616cf564736fcgrigis@icc4dt-lpc-01
`.trim(), "hex");
        const abiV2Abi = `
[{"constant":true,"inputs":[{"internalType":"uint256","name":"limit","type":"u\
int256"}],"name":"squares","outputs":[{"components":[{"internalType":"uint256"\
,"name":"v1","type":"uint256"},{"internalType":"uint256","name":"v2","type":"u\
int256"}],"internalType":"struct ABIv2.S[]","name":"","type":"tuple[]"}],"paya\
ble":false,"stateMutability":"view","type":"function"}]
`.trim();

        // NOTE: Make sure the account privKey is different for each test, to
        // avoid issues with nonces
        const privKey = Buffer.from(
            "e87509a1c067bbde78beb793e6fa76530b6382a4c0241e5e4a9ec0a0f44dc0d3",
            "hex");
        const account = new EvmAccount("test", privKey);
        const contract = new EvmContract("ABIv2", abiV2Bytecode, abiV2Abi);

        // Credit an account so that we can perform actions
        await expectAsync(bevmInstance.creditAccount(
            [admin],
            account,
            WEI_PER_ETHER.mul(5),
        )).toBeResolved();

        // Deploy an ABIv2 contract
        await expectAsync(bevmInstance.deploy(
            [admin],
            1e7,
            1,
            0,
            account,
            contract,
        )).toBeResolved();

        // Retrieve first ten squares
        const [result] = await bevmInstance.call(
            account,
            contract,
            0,
            "squares",
            [10],
        );

        expect(result.length).toEqual(10);

        result.forEach((s: any) => {
            expect(s.v2.eq(s.v1.mul(s.v1))).toBe(true);
        });
    }, 60000); // Extend Jasmine default timeout interval to 1 minute
});

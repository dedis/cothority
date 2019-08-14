import { curve, Point } from "@dedis/kyber";
import Long from "long";
import ByzCoinRPC from "../../src/byzcoin/byzcoin-rpc";
import CoinInstance from "../../src/byzcoin/contracts/coin-instance";
import DarcInstance from "../../src/byzcoin/contracts/darc-instance";
import { IdentityEd25519 } from "../../src/darc";
import Darc from "../../src/darc/darc";
import { Rule } from "../../src/darc/rules";
import ISigner from "../../src/darc/signer";
import SignerEd25519 from "../../src/darc/signer-ed25519";
import { Roster } from "../../src/network";
import { Attribute, Credential, CredentialStruct } from "../../src/personhood/credentials-instance";
import { PopDesc } from "../../src/personhood/proto";
import SpawnerInstance, { SPAWNER_COIN } from "../../src/personhood/spawner-instance";
import { BLOCK_INTERVAL, ROSTER, SIGNER, startConodes } from "../support/conondes";

const ed25519 = curve.newCurve("edwards25519");

describe("SpawnerInstance Tests", () => {
    const roster = ROSTER.slice(0, 4);

    beforeAll(async () => {
        await startConodes();
    });

    it("should create a spawner", async () => {
        const darc = await makeDarc(roster);

        const rpc = await ByzCoinRPC.newByzCoinRPC(roster, darc, BLOCK_INTERVAL);
        const ci = await CoinInstance.spawn(rpc, darc.getBaseID(), [SIGNER], SPAWNER_COIN);
        await ci.mint([SIGNER], Long.fromNumber(10 ** 9, true));
        await ci.update();

        const costs = {
            costCRead: Long.fromNumber(1000),
            costCWrite: Long.fromNumber(1000),
            costCoin: Long.fromNumber(1000),
            costCredential: Long.fromNumber(1000),
            costDarc: Long.fromNumber(1000),
            costParty: Long.fromNumber(1000),
        };

        const params = {bc: rpc, darcID: darc.getBaseID(), signers: [SIGNER], costs, beneficiary: ci.id};
        const si = await SpawnerInstance.spawn(params);

        expect(si.signupCost.toNumber()).toBe(3000);
        await expectAsync(SpawnerInstance.fromByzcoin(rpc, Buffer.from("deadbeef"))).toBeRejected();
        await SpawnerInstance.fromByzcoin(rpc, si.id);

        await si.update();
    });

    it("should spawn a pop party", async () => {
        const darc = await makeDarc(roster);

        const rpc = await ByzCoinRPC.newByzCoinRPC(roster, darc, BLOCK_INTERVAL);
        const ci = await CoinInstance.spawn(rpc, darc.getBaseID(), [SIGNER], SPAWNER_COIN);
        await ci.mint([SIGNER], Long.fromNumber(10 ** 9, true));
        await ci.update();

        const costs = {
            costCRead: Long.fromNumber(1000),
            costCWrite: Long.fromNumber(1000),
            costCoin: Long.fromNumber(1000),
            costCredential: Long.fromNumber(1000),
            costDarc: Long.fromNumber(1000),
            costParty: Long.fromNumber(1000),
        };

        const params = {bc: rpc, darcID: darc.getBaseID(), signers: [SIGNER], costs, beneficiary: ci.id};
        const si = await SpawnerInstance.spawn(params);

        // Get an organiser
        const org = SignerEd25519.fromBytes(Buffer.from([1, 2, 3, 4, 5, 6]));
        const darcOrg = await spawnUserDarc(si, ci, [SIGNER], org.public);
        const orgCred = await si.spawnCredential(ci, [SIGNER], darcOrg.darc.getBaseID(),
            generateCredential(org.public));

        // Get an organiser without key
        const org2 = SignerEd25519.fromBytes(Buffer.from("deadbeef"));
        const darcOrg2 = await spawnUserDarc(si, ci, [SIGNER], org2.public);
        const orgCred2 = await si.spawnCredential(ci, [SIGNER], darcOrg2.darc.getBaseID(), new CredentialStruct());

        // get an attendee
        const attendee = SignerEd25519.fromBytes(Buffer.from([5, 6, 7, 8]));
        const darcAtt = await spawnUserDarc(si, ci, [SIGNER], attendee.public);
        const ciAtt = await si.spawnCoin(ci, [SIGNER], darcAtt.darc.getBaseID(), SPAWNER_COIN);

        // Spawn a pop party
        const desc = new PopDesc({name: "spawned pop party"});

        const popParams = {
            coin: ci,
            desc,
            orgs: [darcOrg.id],
            reward: Long.fromNumber(10000),
            signers: [SIGNER],
        };
        const party = await si.spawnPopParty(popParams);
        expect(party).toBeDefined();

        // must have started
        await expectAsync(party.finalize([org])).toBeRejected();
        expect(() => party.addAttendee(ed25519.point())).toThrow();
        expect(() => party.removeAttendee(ed25519.point())).toThrow();

        await party.activateBarrier([org]);
        // already activated
        await expectAsync(party.activateBarrier([org])).toBeRejected();

        party.addAttendee(org.public);
        party.addAttendee(ed25519.point().pick());
        party.addAttendee(ed25519.point().pick());
        party.addAttendee(attendee.public);

        const pub = ed25519.point().pick();
        party.addAttendee(pub);
        party.removeAttendee(pub);

        // cannot mine before finalizing
        await expectAsync(party.mine(attendee.secret, ciAtt.id)).toBeRejected();
        expect(() => party.finalStatement).toThrow();

        await party.finalize([org]);
        expect(party.popPartyStruct.finalizations).toEqual([org.toString()]);
        // 3 attendees + 1 organiser
        expect(party.popPartyStruct.attendees.keys.length).toBe(3 + 1);
        expect(party.finalStatement).toBeDefined();

        await party.mine(attendee.secret, ciAtt.id);
        expect(party.popPartyStruct.miners.length).toBe(1);

    });

    it("should spawn a rock-paper-scissors game", async () => {
        const darc = await makeDarc(roster);

        const rpc = await ByzCoinRPC.newByzCoinRPC(roster, darc, BLOCK_INTERVAL);
        const ci = await CoinInstance.spawn(rpc, darc.getBaseID(), [SIGNER], SPAWNER_COIN);

        const costs = {
            costCRead: Long.fromNumber(1000),
            costCWrite: Long.fromNumber(1000),
            costCoin: Long.fromNumber(1000),
            costCredential: Long.fromNumber(1000),
            costDarc: Long.fromNumber(1000),
            costParty: Long.fromNumber(1000),
        };

        const params = {bc: rpc, darcID: darc.getBaseID(), signers: [SIGNER], costs, beneficiary: ci.id};
        const si = await SpawnerInstance.spawn(params);

        const stake = Long.fromNumber(100);
        const choice = 2;
        const fillup = Buffer.allocUnsafe(31);

        // fill up too small
        const rpsParams = {desc: "abc", coin: ci, signers: [SIGNER], stake, choice, fillup: Buffer.from([])};
        await expectAsync(si.spawnRoPaSci(rpsParams)).toBeRejected();
        // not enough coins
        rpsParams.fillup = fillup;
        await expectAsync(si.spawnRoPaSci(rpsParams)).toBeRejected();

        await ci.mint([SIGNER], Long.fromNumber(10 ** 9, true));
        await ci.update();
        const game = await si.spawnRoPaSci(rpsParams);

        expect(game.playerChoice).toBe(-1);
        expect(game.stake.value.toNumber()).toBe(stake.toNumber());
    });

    it("should not try to spawn existing instances", async () => {
        const darc = await makeDarc(roster);

        const rpc = await ByzCoinRPC.newByzCoinRPC(roster, darc, BLOCK_INTERVAL);
        const ci = await CoinInstance.spawn(rpc, darc.getBaseID(), [SIGNER], SPAWNER_COIN);
        await ci.mint([SIGNER], Long.fromNumber(10 ** 9, true));
        await ci.update();

        const costs = {
            costCRead: Long.fromNumber(1000),
            costCWrite: Long.fromNumber(1000),
            costCoin: Long.fromNumber(1000),
            costCredential: Long.fromNumber(1000),
            costDarc: Long.fromNumber(1000),
            costParty: Long.fromNumber(1000),
        };

        const params = {bc: rpc, darcID: darc.getBaseID(), signers: [SIGNER], costs, beneficiary: ci.id};
        const si = await SpawnerInstance.spawn(params);

        const user = SignerEd25519.fromBytes(Buffer.from([1, 2, 3, 4, 5, 6]));
        const userDarc = await spawnUserDarc(si, ci, [SIGNER], user.public);
        await expectAsync(spawnUserDarc(si, ci, [SIGNER], user.public)).toBeRejected();

        await si.spawnCredential(ci, [SIGNER], userDarc.darc.getBaseID(),
            generateCredential(user.public));
        await expectAsync(si.spawnCredential(ci, [SIGNER], userDarc.darc.getBaseID(),
            generateCredential(user.public))).toBeRejected();

        const coinInst = await si.spawnCoin(ci, [SIGNER], userDarc.darc.getBaseID(), SPAWNER_COIN);
        await expectAsync(si.spawnCoin(ci, [SIGNER], userDarc.darc.getBaseID(), SPAWNER_COIN))
            .toBeRejected();
    });
});

async function spawnUserDarc(si: SpawnerInstance, ci: CoinInstance, signers: ISigner[], pub: Point):
    Promise<DarcInstance> {
    const id = IdentityEd25519.fromPoint(pub);
    const d = Darc.createBasic([id], [id]);
    return (await si.spawnDarcs(ci, signers, d))[0];
}

async function makeDarc(roster: Roster): Promise<Darc> {
    const darc = ByzCoinRPC.makeGenesisDarc([SIGNER], roster);
    darc.addIdentity("spawn:coin", SIGNER, Rule.OR);
    darc.addIdentity("invoke:coin.mint", SIGNER, Rule.OR);
    darc.addIdentity("invoke:coin.fetch", SIGNER, Rule.OR);
    darc.addIdentity("spawn:spawner", SIGNER, Rule.OR);

    return darc;
}

function generateCredential(pub: Point): CredentialStruct {
    return new CredentialStruct({
        credentials: [new Credential({
            attributes: [new Attribute({
                name: "ed25519",
                value: pub.toProto(),
            })],
            name: "personhood",
        })],
    });
}

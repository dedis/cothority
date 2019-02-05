import Long from 'long';
import { startConodes, ROSTER, SIGNER, BLOCK_INTERVAL } from "../support/conondes";
import SpawnerInstance from '../../src/byzcoin/contracts/spawner-instance';
import ByzCoinRPC from "../../src/byzcoin/byzcoin-rpc";
import Rules from "../../src/darc/rules";
import CoinInstance from '../../src/byzcoin/contracts/coin-instance';
import { curve, Point } from '@dedis/kyber';
import { CredentialStruct, Attribute, Credential } from '../../src/byzcoin/contracts/credentials-instance';
import { PopDesc } from '../../src/byzcoin/contracts/pop-party/proto';
import SignerEd25519 from '../../src/darc/signer-ed25519';
import { Roster } from '../../src/network/proto';
import Darc from '../../src/darc/darc';

const ed25519 = curve.newCurve('edwards25519');

describe('SpawnerInstance Tests', () => {
    const roster = ROSTER.slice(0, 4);

    beforeAll(async () => {
        await startConodes();
    });

    it('should create a spawner', async () => {
        const darc = await makeDarc(roster);

        const rpc = await ByzCoinRPC.newByzCoinRPC(roster, darc, BLOCK_INTERVAL);
        const ci = await CoinInstance.create(rpc, darc.baseID, [SIGNER]);
        await ci.mint([SIGNER], Long.fromNumber(10 ** 9, true));
        await ci.update();

        const costs = {
            costDarc: Long.fromNumber(1000),
            costCoin: Long.fromNumber(1000),
            costCredential: Long.fromNumber(1000),
            costParty: Long.fromNumber(1000),
        };

        const params = { bc: rpc, darcID: darc.baseID, signers: [SIGNER], costs, beneficiary: ci.id };
        const si = await SpawnerInstance.create(params);

        expect(si.signupCost.toNumber()).toBe(3000);
        expectAsync(SpawnerInstance.fromByzcoin(rpc, Buffer.from('deadbeef'))).toBeRejected();
        expectAsync(SpawnerInstance.fromByzcoin(rpc, si.iid)).toBeResolved();

        await si.update();
    });

    it('should spawn a pop party', async () => {
        const darc = await makeDarc(roster);

        const rpc = await ByzCoinRPC.newByzCoinRPC(roster, darc, BLOCK_INTERVAL);
        const ci = await CoinInstance.create(rpc, darc.baseID, [SIGNER]);
        await ci.mint([SIGNER], Long.fromNumber(10 ** 9, true));
        await ci.update();

        const costs = {
            costDarc: Long.fromNumber(1000),
            costCoin: Long.fromNumber(1000),
            costCredential: Long.fromNumber(1000),
            costParty: Long.fromNumber(1000),
        };

        const params = { bc: rpc, darcID: darc.baseID, signers: [SIGNER], costs, beneficiary: ci.id };
        const si = await SpawnerInstance.create(params);

        // Get an organiser
        const org = SignerEd25519.fromBytes(Buffer.from([1, 2, 3, 4, 5, 6]));
        const darcOrg = await si.createUserDarc(ci, [SIGNER], org.public, 'org');
        const orgCred = await si.createCredential(ci, [SIGNER], darcOrg.getDarc().baseID, generateCredential(org.public));

        // Get an organiser without key
        const org2 = SignerEd25519.fromBytes(Buffer.from('deadbeef'));
        const darcOrg2 = await si.createUserDarc(ci, [SIGNER], org2.public, 'org2');
        const orgCred2 = await si.createCredential(ci, [SIGNER], darcOrg2.getDarc().baseID, new CredentialStruct());

        // get an attendee
        const attendee = SignerEd25519.fromBytes(Buffer.from([5, 6, 7, 8]));
        const darcAtt = await si.createUserDarc(ci, [SIGNER], attendee.public, 'attendee');
        const ciAtt = await si.createCoin(ci, [SIGNER], darcAtt.getDarc().baseID);
        
        // Spawn a pop party
        const desc = new PopDesc({ name: 'spawned pop party' });

        const popParams = { coin: ci, signers: [SIGNER], orgs: [orgCred, orgCred2], desc, reward: Long.fromNumber(10000) };
        expectAsync(si.createPopParty(popParams)).toBeRejected();

        popParams.orgs = [orgCred];
        const party = await si.createPopParty(popParams);
        expect(party).toBeDefined();

        // must have started
        expectAsync(party.finalize([org])).toBeRejected();
        expect(() => party.addAttendee(ed25519.point())).toThrow();
        expect(() => party.removeAttendee(ed25519.point())).toThrow();

        await party.activateBarrier([org]);
        // already activated
        expectAsync(party.activateBarrier([org])).toBeRejected();

        party.addAttendee(ed25519.point().pick());
        party.addAttendee(ed25519.point().pick());
        party.addAttendee(attendee.public);

        const pub = ed25519.point().pick();
        party.addAttendee(pub);
        party.removeAttendee(pub);

        // cannot mine before finalizing
        expectAsync(party.mine(attendee.secret, ciAtt.id)).toBeRejected();
        expect(() => party.finalStatement).toThrow();

        await party.finalize([org]);
        expect(party.data.finalizations).toEqual([org.toString()]);
        // 3 attendees + 1 organiser
        expect(party.data.attendees.keys.length).toBe(3 + 1);
        expect(party.finalStatement).toBeDefined();

        await party.mine(attendee.secret, ciAtt.id);
        expect(party.data.miners.length).toBe(1);
    });

    it('should spawn a rock-paper-scisors game', async () => {
        const darc = await makeDarc(roster);

        const rpc = await ByzCoinRPC.newByzCoinRPC(roster, darc, BLOCK_INTERVAL);
        const ci = await CoinInstance.create(rpc, darc.baseID, [SIGNER]);

        const costs = {
            costDarc: Long.fromNumber(1000),
            costCoin: Long.fromNumber(1000),
            costCredential: Long.fromNumber(1000),
            costParty: Long.fromNumber(1000),
        };

        const params = { bc: rpc, darcID: darc.baseID, signers: [SIGNER], costs, beneficiary: ci.id };
        const si = await SpawnerInstance.create(params);

        const stake = Long.fromNumber(100);
        const choice = 2;
        const fillup = Buffer.allocUnsafe(31);

        // fill up too small
        const rpsParams = { desc: 'abc', coin: ci, signers: [SIGNER], stake, choice, fillup: Buffer.from([]) };
        expectAsync(si.createRoPaSci(rpsParams)).toBeRejected();
        // not enough coins
        rpsParams.fillup = fillup;
        expectAsync(si.createRoPaSci(rpsParams)).toBeRejected();

        await ci.mint([SIGNER], Long.fromNumber(10 ** 9, true));
        await ci.update();
        const game = await si.createRoPaSci(rpsParams);

        expect(game.playerChoice).toBe(-1);
        expect(game.stake.value.toNumber()).toBe(stake.toNumber());
    });

    it('should not try to create existing instances', async () => {
        const darc = await makeDarc(roster);

        const rpc = await ByzCoinRPC.newByzCoinRPC(roster, darc, BLOCK_INTERVAL);
        const ci = await CoinInstance.create(rpc, darc.baseID, [SIGNER]);
        await ci.mint([SIGNER], Long.fromNumber(10 ** 9, true));
        await ci.update();

        const costs = {
            costDarc: Long.fromNumber(1000),
            costCoin: Long.fromNumber(1000),
            costCredential: Long.fromNumber(1000),
            costParty: Long.fromNumber(1000),
        };

        const params = { bc: rpc, darcID: darc.baseID, signers: [SIGNER], costs, beneficiary: ci.id };
        const si = await SpawnerInstance.create(params);

        const user = SignerEd25519.fromBytes(Buffer.from([1, 2, 3, 4, 5, 6]));
        const userDarc = await si.createUserDarc(ci, [SIGNER], user.public, 'org');
        const userDarc2 = await si.createUserDarc(ci, [SIGNER], user.public, 'org');
        expect(userDarc.getDarc().id).toEqual(userDarc2.getDarc().id);

        const userCred = await si.createCredential(ci, [SIGNER], userDarc.getDarc().baseID, generateCredential(user.public));
        const userCred2 = await si.createCredential(ci, [SIGNER], userDarc.getDarc().baseID, generateCredential(user.public));
        expect(userCred.darcID).toEqual(userCred2.darcID);

        const userCoin = await si.createCoin(ci, [SIGNER], userDarc.getDarc().baseID);
        const userCoin2 = await si.createCoin(ci, [SIGNER], userDarc.getDarc().baseID);
        expect(userCoin.id).toEqual(userCoin2.id);
    });
});

async function makeDarc(roster: Roster): Promise<Darc> {
    const darc = ByzCoinRPC.makeGenesisDarc([SIGNER], roster);
    darc.addIdentity('spawn:coin', SIGNER, Rules.OR);
    darc.addIdentity('invoke:coin.mint', SIGNER, Rules.OR);
    darc.addIdentity('invoke:coin.fetch', SIGNER, Rules.OR);
    darc.addIdentity('spawn:spawner', SIGNER, Rules.OR);
    
    return darc;
}

function generateCredential(pub: Point): CredentialStruct {
    return new CredentialStruct({
        credentials: [new Credential({
            name: 'personhood',
            attributes: [new Attribute({
                name: 'ed25519',
                value: pub.toProto(),
            })]
        })],
    });
}

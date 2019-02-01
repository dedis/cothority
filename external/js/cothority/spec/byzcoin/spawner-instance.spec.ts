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

const ed25519 = curve.newCurve('edwards25519');

describe('SpawnerInstance Tests', () => {
    const admin = SIGNER;
    const roster = ROSTER.slice(0, 4);

    beforeAll(async () => {
        await startConodes();
    });

    it('should spawn a pop party', async () => {
        const darc = ByzCoinRPC.makeGenesisDarc([admin], roster);
        darc.addIdentity('spawn:coin', admin, Rules.OR);
        darc.addIdentity('invoke:coin.mint', admin, Rules.OR);
        darc.addIdentity('invoke:coin.fetch', admin, Rules.OR);
        darc.addIdentity('spawn:spawner', admin, Rules.OR);

        const rpc = await ByzCoinRPC.newByzCoinRPC(roster, darc, BLOCK_INTERVAL);
        const ci = await CoinInstance.create(rpc, darc.baseID, [admin]);
        await ci.mint([admin], Long.fromNumber(10 ** 9, true));
        await ci.update();

        const costs = {
            costDarc: Long.fromNumber(1000),
            costCoin: Long.fromNumber(1000),
            costCredential: Long.fromNumber(1000),
            costParty: Long.fromNumber(1000),
        };

        const si = await SpawnerInstance.create(rpc, darc.baseID, [admin], costs, ci.id);

        // Get an organiser
        const org = SignerEd25519.fromBytes(Buffer.from([1, 2, 3, 4, 5, 6]));
        const darcOrg = await si.createUserDarc(ci, [admin], org.public, 'org');
        const orgCred = await si.createCredential(ci, [admin], darcOrg.darc.baseID, generateCredential(org.public));

        // get an attendee
        const attendee = SignerEd25519.fromBytes(Buffer.from([5, 6, 7, 8]));
        const darcAtt = await si.createUserDarc(ci, [admin], attendee.public, 'attendee');
        const ciAtt = await si.createCoin(ci, [admin], darcAtt.darc.baseID);
        
        // Spawn a pop party
        const desc = new PopDesc({ name: 'spawned pop party' });
        const party = await si.createPopParty(ci, [admin], [orgCred], desc, Long.fromNumber(10000));
        expect(party).toBeDefined();

        await party.activateBarrier(org);
        await party.addAttendee(ed25519.point().pick());
        await party.addAttendee(ed25519.point().pick());
        await party.addAttendee(attendee.public);

        const pub = ed25519.point().pick();
        await party.addAttendee(pub);
        await party.delAttendee(pub);

        await party.finalize(org);
        expect(party.data.finalizations).toEqual([org.toString()]);
        // 3 attendees + 1 organiser
        expect(party.data.attendees.keys.length).toBe(3 + 1);

        await party.mine(admin, attendee.secret, ciAtt.id);
        expect(party.data.miners.length).toBe(1);
    });

    it('should spawn a rock-paper-scisors game', async () => {
        const darc = ByzCoinRPC.makeGenesisDarc([admin], roster);
        darc.addIdentity('spawn:coin', admin, Rules.OR);
        darc.addIdentity('invoke:coin.mint', admin, Rules.OR);
        darc.addIdentity('invoke:coin.fetch', admin, Rules.OR);
        darc.addIdentity('spawn:spawner', admin, Rules.OR);

        const rpc = await ByzCoinRPC.newByzCoinRPC(roster, darc, BLOCK_INTERVAL);
        const ci = await CoinInstance.create(rpc, darc.baseID, [admin]);
        await ci.mint([admin], Long.fromNumber(10 ** 9, true));
        await ci.update();

        const costs = {
            costDarc: Long.fromNumber(1000),
            costCoin: Long.fromNumber(1000),
            costCredential: Long.fromNumber(1000),
            costParty: Long.fromNumber(1000),
        };

        const si = await SpawnerInstance.create(rpc, darc.baseID, [admin], costs, ci.id);

        const stake = Long.fromNumber(100);
        const choice = 2;
        const fillup = Buffer.allocUnsafe(31);
        const game = await si.createRoPaSci('abc', ci, admin, stake, choice, fillup);

        expect(game.playerChoice).toBe(-1);
        expect(game.stake.value.toNumber()).toBe(stake.toNumber());
    });
});

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

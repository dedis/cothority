import {CredentialStruct, CredentialInstance} from "~/lib/cothority/byzcoin/contracts/CredentialInstance";

const crypto = require("crypto-browserify");

import {ByzCoinRPC} from "~/lib/cothority/byzcoin/ByzCoinRPC";
import {Instance} from "~/lib/cothority/byzcoin/Instance";
import {Darc, Rule, Rules} from "~/lib/cothority/darc/Darc";
import {Argument, ClientTransaction, InstanceID, Instruction} from "~/lib/cothority/byzcoin/ClientTransaction";
import {Signer} from "~/lib/cothority/darc/Signer";
import * as Long from "long";
import {Coin, CoinInstance} from "~/lib/cothority/byzcoin/contracts/CoinInstance";
import {Proof} from "~/lib/cothority/byzcoin/Proof";
import {Root} from "~/lib/cothority/protobuf/Root";
import {DarcInstance} from "~/lib/cothority/byzcoin/contracts/DarcInstance";
import {IdentityEd25519} from "~/lib/cothority/darc/IdentityEd25519";
import {Buffer} from "buffer";
import {Public} from "~/lib/KeyPair";
import {PopDesc, PopPartyInstance, PopPartyStruct} from "~/lib/cothority/byzcoin/contracts/PopPartyInstance";
import {IdentityDarc} from "~/lib/cothority/darc/IdentityDarc";
import {Contact} from "~/lib/Contact";
import {Log} from "~/lib/Log";
import {RoPaSciInstance, RoPaSciStruct} from "~/lib/cothority/byzcoin/contracts/RoPaSciInstance";

let coinName = new Buffer(32);
coinName.write("SpawnerCoin");
export let SpawnerCoin = new InstanceID(coinName);

export class SpawnerInstance {
    static readonly contractID = "spawner";

    /**
     * Creates a new SpawnerInstance
     * @param {ByzCoinRPC} bc - the ByzCoinRPC instance
     * @param {Instance} iid - the complete instance
     * @param {Spawner} spwaner - parameters for the spawner: costs and names
     */
    constructor(public bc: ByzCoinRPC, public iid: InstanceID, public spawner: Spawner) {
    }

    /**
     * Update the data of this instance
     *
     * @return {Promise<SpawnerInstance>} - a promise that resolves once the data
     * is up-to-date
     */
    async update(): Promise<SpawnerInstance> {
        let proof = await this.bc.getProof(this.iid);
        this.spawner = Spawner.fromProto(proof.value);
        return this;
    }

    async createUserDarc(coin: CoinInstance, signers: Signer[], pubKey: any,
                         alias: string):
        Promise<DarcInstance> {
        let d = SpawnerInstance.prepareUserDarc(pubKey, alias);
        let pr = await this.bc.getProof(new InstanceID(d.getBaseId()));
        if (pr.matches) {
            Log.lvl2("this darc is already registerd");
            return DarcInstance.fromProof(this.bc, pr);
        }

        let ctx = new ClientTransaction([
            Instruction.createInvoke(coin.iid,
                "fetch", [
                    new Argument("coins", Buffer.from(this.spawner.costDarc.value.toBytesLE()))
                ]),
            Instruction.createSpawn(this.iid,
                DarcInstance.contractID, [
                    new Argument("darc", d.toProto())
                ])]);
        await ctx.signBy([signers, []], this.bc);
        await this.bc.sendTransactionAndWait(ctx);
        return DarcInstance.fromByzcoin(this.bc, new InstanceID(d.getBaseId()));
    }

    async createCoin(coin: CoinInstance, signers: Signer[], darcID: Buffer,
                     balance: Long = Long.fromNumber(0)):
        Promise<CoinInstance> {
        let pr = await this.bc.getProof(SpawnerInstance.coinIID(darcID));
        if (pr.matches) {
            Log.lvl2("this coin is already registerd");
            return CoinInstance.fromProof(this.bc, pr);
        }

        let valueBuf = this.spawner.costCoin.value.add(balance).toBytesLE();
        let ctx = new ClientTransaction([
            Instruction.createInvoke(coin.iid,
                "fetch", [
                    new Argument("coins", Buffer.from(valueBuf))
                ]),
            Instruction.createSpawn(this.iid,
                CoinInstance.contractID, [
                    new Argument("coinName", SpawnerCoin.iid),
                    new Argument("darcID", darcID),
                ])]);
        await ctx.signBy([signers, []], this.bc);
        await this.bc.sendTransactionAndWait(ctx);
        return CoinInstance.fromByzcoin(this.bc, SpawnerInstance.coinIID(darcID));
    }

    async createCredential(coin: CoinInstance, signers: Signer[], darcID: Buffer,
                           cred: CredentialStruct):
        Promise<CredentialInstance> {
        let pr = await this.bc.getProof(SpawnerInstance.credentialIID(darcID));
        if (pr.matches) {
            Log.lvl2("this credential is already registerd");
            return CredentialInstance.fromProof(this.bc, pr);
        }

        let valueBuf = this.spawner.costCred.value.toBytesLE();
        let ctx = new ClientTransaction([
            Instruction.createInvoke(coin.iid,
                "fetch", [
                    new Argument("coins", Buffer.from(valueBuf))
                ]),
            Instruction.createSpawn(this.iid,
                CredentialInstance.contractID, [
                    new Argument("darcID", darcID),
                    new Argument("credential", cred.toProto()),
                ])]);
        await ctx.signBy([signers, []], this.bc);
        await this.bc.sendTransactionAndWait(ctx);
        return CredentialInstance.fromByzcoin(this.bc, SpawnerInstance.credentialIID(darcID));
    }

    async createPopParty(coin: CoinInstance, signers: Signer[],
                         orgs: Contact[],
                         descr: PopDesc, reward: Long):
        Promise<PopPartyInstance> {

        // Verify that all organizers have published their personhood public key
        let creds = await Promise.all(orgs.map(org => CredentialInstance.fromByzcoin(this.bc, org.credentialIID)));
        await Promise.all(creds.map(async (cred: CredentialInstance, i: number) => {
            let pop = cred.getAttribute("personhood", "ed25519");
            if (!pop) {
                return Promise.reject("Org " + orgs[i].alias + " didn't publish his personhood key");
            }
        }));

        let orgDarcIDs = orgs.map(org => org.darcInstance.iid);
        let valueBuf = this.spawner.costDarc.value.add(this.spawner.costParty.value).toBytesLE();
        let orgDarc = SpawnerInstance.preparePartyDarc(orgDarcIDs, "party-darc " + descr.name);
        let ctx = new ClientTransaction([
            Instruction.createInvoke(coin.iid,
                "fetch", [
                    new Argument("coins", Buffer.from(valueBuf))
                ]),
            Instruction.createSpawn(this.iid,
                DarcInstance.contractID, [
                    new Argument("darc", orgDarc.toProto()),
                ]),
            Instruction.createSpawn(this.iid,
                PopPartyInstance.contractID, [
                    new Argument("darcID", orgDarc.getBaseId()),
                    new Argument("description", descr.toProto()),
                    new Argument("miningReward", Buffer.from(reward.toBytesLE()))
                ])]);
        await ctx.signBy([signers, [], []], this.bc);
        await this.bc.sendTransactionAndWait(ctx);
        let ppi = PopPartyInstance.fromByzcoin(this.bc, new InstanceID(ctx.instructions[2].deriveId("")));
        return ppi;
    }

    async createRoPaSci(desc: string, coin: CoinInstance, signer: Signer,
                        stake: Long, choice: number, fillup: Buffer):
        Promise<RoPaSciInstance> {
        if (fillup.length != 31){
            return Promise.reject("need exactly 31 bytes for fillUp");
        }
        let c = new Coin({name: coin.coin.name.iid, value: stake.add(this.spawner.costRoPaSci.value) });
        if (coin.coin.value.lessThan(c.value)){
            return Promise.reject("account balance not high enough for that stake");
        }
        let fph = crypto.createHash("sha256");
        fph.update(Buffer.from([choice % 3]));
        fph.update(fillup);
        let rps = new RoPaSciStruct(desc, c, fph.digest(), -1, -1, null);

        let ctx = new ClientTransaction([
            Instruction.createInvoke(coin.iid,
                "fetch", [
                    new Argument("coins", Buffer.from(c.value.toBytesLE()))
                ]),
            Instruction.createSpawn(this.iid,
                RoPaSciInstance.contractID, [
                    new Argument("struct", rps.toProto())
                ])]);
        await ctx.signBy([[signer], []], this.bc);
        await this.bc.sendTransactionAndWait(ctx);
        let rpsi = await RoPaSciInstance.fromByzcoin(this.bc, new InstanceID(ctx.instructions[1].deriveId()));
        rpsi.fillUp = fillup;
        rpsi.firstMove = choice;
        return rpsi;
    }

    get signupCost(): Long {
        return this.spawner.costCoin.value.add(this.spawner.costDarc.value).add(this.spawner.costCred.value);
    }

    static async create(bc: ByzCoinRPC, iid: InstanceID, signers: Signer[],
                        costDarc: Long, costCoin: Long,
                        costCred: Long, costParty: Long,
                        beneficiary: InstanceID): Promise<SpawnerInstance> {
        let args =
            [["costDarc", costDarc],
                ["costCoin", costCoin],
                ["costCredential", costCred],
                ["costParty", costParty]].map(cost =>
                new Argument(<string>cost[0],
                    Coin.create(SpawnerCoin, <Long>cost[1]).toProto()));
        args.push(new Argument("beneficiary", beneficiary.iid));
        let inst = Instruction.createSpawn(iid, this.contractID, args);
        let ctx = new ClientTransaction([inst]);
        await ctx.signBy([signers], bc);
        await bc.sendTransactionAndWait(ctx, 5);
        return this.fromByzcoin(bc, new InstanceID(inst.deriveId()));
    }

    static async fromProof(bc: ByzCoinRPC, p: Proof): Promise<SpawnerInstance> {
        await p.matchOrFail(SpawnerInstance.contractID);
        return new SpawnerInstance(bc, p.requestedIID,
            Spawner.fromProto(p.value));
    }

    /**
     * Initializes using an existing coinInstance from ByzCoin
     * @param bc
     * @param instID
     */
    static async fromByzcoin(bc: ByzCoinRPC, iid: InstanceID): Promise<SpawnerInstance> {
        return SpawnerInstance.fromProof(bc, await bc.getProof(iid));
    }

    static prepareUserDarc(pubKey: Public, alias: string): Darc {
        let id = new IdentityEd25519(pubKey.point);
        let r = Rules.fromOwnersSigners([id], [id]);
        r.list.push(Rule.fromIdentities("invoke:update", [id], "&"));
        r.list.push(Rule.fromIdentities("invoke:fetch", [id], "&"));
        r.list.push(Rule.fromIdentities("invoke:transfer", [id], "&"));
        return Darc.fromRulesDesc(r, "user " + alias);
    }

    static preparePartyDarc(darcIDs: InstanceID[], desc: string): Darc {
        let ids = darcIDs.map(di => new IdentityDarc(di));
        let r = Rules.fromOwnersSigners(ids, ids);
        r.list.push(Rule.fromIdentities("invoke:barrier", ids, "|"));
        r.list.push(Rule.fromIdentities("invoke:finalize", ids, "|"));
        r.list.push(Rule.fromIdentities("invoke:addParty", ids, "|"));
        return Darc.fromRulesDesc(r, desc);
    }

    static credentialIID(darcBaseID: Buffer): InstanceID {
        let h = crypto.createHash("sha256");
        h.update(Buffer.from("credential"));
        h.update(darcBaseID);
        return new InstanceID(h.digest());
    }

    static coinIID(darcBaseID: Buffer): InstanceID {
        let h = crypto.createHash("sha256");
        h.update(Buffer.from("coin"));
        h.update(darcBaseID);
        return new InstanceID(h.digest());
    }
}

export class Spawner {
    static readonly protoName = "personhood.SpawnerStruct";

    costDarc: Coin;
    costCoin: Coin;
    costCred: Coin;
    costParty: Coin;
    costRoPaSci: Coin;
    costPoll: Coin;
    beneficiary: InstanceID;

    constructor(obj: any) {
        this.costDarc = obj.costdarc;
        this.costCoin = obj.costcoin;
        this.costCred = obj.costcredential;
        this.costParty = obj.costparty;
        this.costRoPaSci = obj.costropasci;
        this.costPoll = obj.costpoll;
        this.beneficiary = obj.beneficiary;
    }

    toObject(): object {
        return {
            costdarc: this.costDarc,
            costcoin: this.costCoin,
            costcredential: this.costCred,
            costparty: this.costParty,
            costropasci: this.costRoPaSci,
            costpoll: this.costPoll,
            beneficiary: this.beneficiary ? this.beneficiary.iid : Buffer.alloc(32),
        }
    }

    static fromProto(buf: Buffer): Spawner {
        return new Spawner(Root.lookup("SpawnerStruct").decode(buf));
    }
}


import {ByzCoinRPC} from "~/lib/cothority/byzcoin/ByzCoinRPC";
import {Argument, ClientTransaction, InstanceID, Instruction} from "~/lib/cothority/byzcoin/ClientTransaction";
import {Proof} from "~/lib/cothority/byzcoin/Proof";
import {objToProto, Root} from "~/lib/cothority/protobuf/Root";
import * as Long from "long";
import {KeyPair, Public} from "~/lib/KeyPair";
import {Log} from "~/lib/Log";
import {Buffer} from "buffer";
import {DarcInstance} from "~/lib/cothority/byzcoin/contracts/DarcInstance";
import {Signer} from "~/lib/cothority/darc/Signer";
import {CoinInstance} from "~/lib/cothority/byzcoin/contracts/CoinInstance";
import {Contact} from "~/lib/Contact";
import {Data} from "~/lib/Data";
import {promises} from "fs";
import {Sign} from "~/lib/RingSig";
import {SpawnerInstance} from "~/lib/cothority/byzcoin/contracts/SpawnerInstance";
import {Darc} from "~/lib/cothority/darc/Darc";
import {sprintf} from "sprintf-js";
import {Party} from "~/lib/Party";
import {BasicInstance} from "~/lib/cothority/byzcoin/contracts/Instance";
import {CredentialInstance} from "~/lib/cothority/byzcoin/contracts/CredentialInstance";
import {msgFailed} from "~/lib/ui/messages";

export class PopPartyInstance extends BasicInstance {
    static readonly contractID = "popParty";

    public tmpAttendees: Public[] = [];
    public popPartyStruct: PopPartyStruct;

    constructor(public bc: ByzCoinRPC, p: Proof | any = null) {
        super(bc, PopPartyInstance.contractID, p);
        this.popPartyStruct = PopPartyStruct.fromProto(this.data);
    }

    async fetchOrgKeys(): Promise<Public[]> {
        let piDarc = await DarcInstance.fromByzcoin(this.bc, this.darcID);
        let exprOrgs = piDarc.darc.rules.list.find(l => l.action == "invoke:finalize").expr;
        let orgDarcs = exprOrgs.toString().split(" | ");
        let orgPers: Public[] = [];
        for (let i = 0; i < orgDarcs.length; i++) {
            // Remove leading "darc:" from expression
            let orgDarc = orgDarcs[i].substr(5);
            let orgCred = SpawnerInstance.credentialIID(Buffer.from(orgDarc, 'hex'));
            Log.lvl2("Searching personhood-identity of organizer", orgDarc, orgCred);
            let cred = await CredentialInstance.fromByzcoin(this.bc, orgCred);
            let credPers = cred.getAttribute("personhood", "ed25519");
            if (!credPers) {
                return Promise.reject("found organizer without personhood credential");
            }
            orgPers.push(Public.fromBuffer(credPers));
        }
        return orgPers;
    }

    async getFinalStatement(): Promise<FinalStatement> {
        if (this.popPartyStruct.state != Party.Finalized) {
            return Promise.reject("this party is not finalized yet");
        }
        return new FinalStatement(this.popPartyStruct.description, this.popPartyStruct.attendees);
    }

    async activateBarrier(org: Signer): Promise<number> {
        if (this.popPartyStruct.state != Party.PreBarrier) {
            return Promise.reject("barrier point has already been passed");
        }

        let ctx = new ClientTransaction([
            Instruction.createInvoke(this.iid,
                "barrier", [])]);
        await ctx.signBy([[org]], this.bc);
        await this.bc.sendTransactionAndWait(ctx);
        await this.update();
        return this.popPartyStruct.state;
    }

    async addAttendee(attendee: Public): Promise<number> {
        if (this.popPartyStruct.state != Party.Scanning) {
            return Promise.reject("party is not in attendee-adding mode");
        }

        if (this.tmpAttendees.findIndex(pub => pub.point.equal(attendee.point)) >= 0) {
            return Promise.reject("already have this attendee");
        }

        this.tmpAttendees.push(attendee);
    }

    async delAttendee(attendee: Public): Promise<number> {
        if (this.popPartyStruct.state != Party.Scanning) {
            return Promise.reject("party is not in attendee-adding mode");
        }

        let i = this.tmpAttendees.findIndex(pub => pub.point.equal(attendee.point));
        if (i == -1) {
            return Promise.reject("unknown attendee");
        }
        this.tmpAttendees.splice(i, 1);
        return this.tmpAttendees.length;
    }

    async finalize(org: Signer): Promise<number> {
        if (this.popPartyStruct.state != Party.Scanning) {
            return Promise.reject("party did not pass barrier-point yet");
        }
        this.tmpAttendees.sort((a, b) => Buffer.compare(a.toBuffer(), b.toBuffer()));
        this.popPartyStruct.attendees.keys = this.tmpAttendees;

        let ctx = new ClientTransaction([
            Instruction.createInvoke(this.iid,
                "finalize", [
                    new Argument("attendees", this.popPartyStruct.attendees.toProtobuf())
                ])]);
        await ctx.signBy([[org]], this.bc);
        await this.bc.sendTransactionAndWait(ctx);
        await this.update();
        return this.popPartyStruct.state;
    }

    async update(): Promise<PopPartyInstance> {
        let proof = await this.bc.getProof(this.iid);
        await proof.matchOrFail(PopPartyInstance.contractID);
        this.popPartyStruct = PopPartyStruct.fromProto(proof.value);
        this.data = proof.value;
        if (this.popPartyStruct.state == Party.Scanning &&
            this.tmpAttendees.length == 0) {
            this.tmpAttendees = await this.fetchOrgKeys();
        }
        return this;
    }

    setProgress(text: string = "", percentage: number = 0){
        Log.lvl1("dummy-process:", text, percentage);
    }

    async mineFromData(att: Data, setProgress: Function = this.setProgress): Promise<any> {
        if (att.coinInstance) {
            setProgress("Mining", 50);
            await this.mine(att.keyPersonhood, att.coinInstance.iid);
            setProgress("Mining", 100);
        } else {
            let newDarc = SpawnerInstance.prepareUserDarc(att.keyIdentity._public, att.alias);
            setProgress("Creating coin & darc", 33);
            await this.mine(att.keyPersonhood, null, newDarc);
            att.coinInstance = await CoinInstance.fromByzcoin(this.bc, SpawnerInstance.coinIID(newDarc.getBaseId()));
            att.darcInstance = await DarcInstance.fromProof(this.bc,
                await this.bc.getProof(new InstanceID(newDarc.getBaseId())));
            setProgress("Creating credentials", 67);
            att.credentialInstance = await att.createUserCredentials();
            await att.save()
            setProgress("Registered user", 100);
        }
    }

    async mine(att: KeyPair, coin: InstanceID, newDarc: Darc = null): Promise<any> {
        if (this.popPartyStruct.state != Party.Finalized) {
            return Promise.reject("cannot mine on a non-finalized party");
        }
        let lrs = await Sign(Buffer.from("mine"), this.popPartyStruct.attendees.keys, this.iid.iid, att._private);
        let args = [new Argument("lrs", lrs.encode())];
        if (coin) {
            args.push(new Argument("coinIID", coin.iid));
        } else if (newDarc) {
            args.push(new Argument("newDarc", newDarc.toProto()));
        } else {
            return Promise.reject("neither coin nor darc given");
        }
        let ctx = new ClientTransaction([
            Instruction.createInvoke(this.iid,
                "mine", args)]);
        await this.bc.sendTransactionAndWait(ctx);
        await this.update();
    }

    static fromObject(bc: ByzCoinRPC, obj: any): PopPartyInstance {
        return new PopPartyInstance(bc, obj);
    }

    static async fromByzcoin(bc: ByzCoinRPC, iid: InstanceID): Promise<PopPartyInstance> {
        return new PopPartyInstance(bc, await bc.getProof(iid));
    }
}

export class PopPartyStruct {
    static readonly protoName = "personhood.PopPartyStruct";

    constructor(public state: number,
                public organizers: number,
                public finalizations: string[],
                public description: PopDesc,
                public attendees: Attendees,
                public miners: LRSTag[],
                public miningReward: Long,
                public previous: InstanceID,
                public next: InstanceID) {
    }

    toObject(): object {
        let o = {
            state: this.state,
            organizers: this.organizers,
            finalizations: this.finalizations,
            description: this.description.toObject(),
            attendees: this.attendees.toObject(),
            miners: this.miners,
            miningreward: this.miningReward,
            previous: null,
            next: null
        };
        this.previous ? o.previous = this.previous.iid : null;
        this.next ? o.previous = this.next.iid : null;
        return o;
    }

    toProto(): Buffer {
        return objToProto(this.toObject(), PopPartyStruct.protoName);
    }

    static fromProto(buf: Buffer): PopPartyStruct {
        let pps = Root.lookup(PopPartyStruct.protoName).decode(buf);
        let prev = (pps.previous && pps.previous.length == 32) ? new InstanceID(pps.previous) : null;
        let next = (pps.next && pps.next.length == 32) ? new InstanceID(pps.next) : null;
        return new PopPartyStruct(pps.state, pps.organizers, pps.finalizations,
            PopDesc.fromObject(pps.description),
            Attendees.fromObject(pps.attendees),
            pps.miners.map(m => LRSTag.fromObject(m)),
            pps.miningreward, prev, next);
    }
}

export class FinalStatement {
    static readonly protoName = "personhood.FinalStatement";

    constructor(public desc: PopDesc,
                public attendees: Attendees) {
    }

    toObject(): object {
        return {
            desc: this.desc.toObject(),
            attendees: this.attendees.toObject(),
        }
    }

    static fromObject(o: any): FinalStatement {
        return new FinalStatement(PopDesc.fromObject(o.popdesc), Attendees.fromObject(o.attendees));
    }
}

export class PopDesc {
    static readonly protoName = "personhood.PopDesc";

    constructor(public name: string,
                public purpose: string,
                public dateTime: Long,
                public location: string) {
    }

    toObject(): object {
        return {
            name: this.name,
            purpose: this.purpose,
            datetime: this.dateTime,
            location: this.location,
        };
    }

    toProto(): Buffer {
        return objToProto(this.toObject(), PopDesc.protoName);
    }

    get dateString(): string {
        return new Date(this.dateTime.toNumber()).toString().replace(/ GMT.*/, "");
    }

    get uniqueName(): string {
        let d = new Date(this.dateTime.toNumber());
        return sprintf("%02d-%02d-%02d %02d:%02d", d.getFullYear() % 100, d.getMonth() + 1, d.getDate(),
            d.getHours(), d.getMinutes())
    }

    static fromObject(o: any): PopDesc {
        return new PopDesc(o.name, o.purpose, o.datetime, o.location);
    }
}

export class Attendees {
    static readonly protoName = "personhood.Attendees";

    constructor(public keys: Public[]) {
    }

    toObject(): object {
        return {
            keys: this.keys.map(k => k.toBuffer())
        };
    }

    toProtobuf(): Buffer {
        return objToProto(this.toObject(), Attendees.protoName);
    }

    static fromObject(o: any): Attendees {
        return new Attendees(o.keys.map(k => Public.fromBuffer(k)));
    }
}

export class LRSTag {
    static readonly protoName = "personhood.LRSTag";

    constructor(public tag: Buffer) {
    }

    static fromObject(o: any): LRSTag {
        return new LRSTag(o.tag);
    }
}
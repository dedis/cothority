import { Point, Scalar, PointFactory, sign } from "@dedis/kyber";
import ByzCoinRPC from "../../byzcoin-rpc";
import ClientTransaction, { Argument, Instruction } from "../../client-transaction";
import DarcInstance from "../darc-instance";
import Signer from "../../../darc/signer";
import SpawnerInstance from "../spawner-instance";
import CredentialInstance from "../credentials-instance";
import Instance from "../../instance";
import Log from '../../../log';
import { PopPartyStruct, FinalStatement } from "./proto";

const { anon } = sign;

export class PopPartyInstance {
    static readonly contractID = "popParty";
    static readonly PreBarrier = 1;
    static readonly Scanning = 2;
    static readonly Finalized = 3;

    private rpc: ByzCoinRPC;
    private instance: Instance;
    private tmpAttendees: Point[] = [];
    private popPartyStruct: PopPartyStruct;

    constructor(public bc: ByzCoinRPC, inst: Instance) {
        this.rpc = bc;
        this.instance = inst;
        this.popPartyStruct = PopPartyStruct.decode(this.instance.data);
    }

    /**
     * Getter for the party data
     * 
     * @returns the data struct
     */
    get data(): PopPartyStruct {
        return this.popPartyStruct;
    }

    /**
     * Getter for the final statement. It throws if the party
     * is not finalized.
     * 
     * @returns the final statement
     */
    get finalStatement(): FinalStatement {
        if (this.popPartyStruct.state !== PopPartyInstance.Finalized) {
            throw new Error("this party is not finalized yet");
        }

        return new FinalStatement({
            desc: this.popPartyStruct.description,
            attendees: this.popPartyStruct.attendees,
        });
    }

    /**
     * Add an attendee to the party
     * 
     * @param attendee The public key of the attendee
     */
    addAttendee(attendee: Point): void {
        if (this.popPartyStruct.state !== PopPartyInstance.Scanning) {
            throw new Error("party is not in attendee-adding mode");
        }

        if (this.tmpAttendees.findIndex(pub => pub.equals(attendee)) === -1) {
            this.tmpAttendees.push(attendee);
        }
    }

    /**
     * Remove an attendee from the party
     * 
     * @param attendee The public key of the attendee
     */
    removeAttendee(attendee: Point): number {
        if (this.popPartyStruct.state !== PopPartyInstance.Scanning) {
            throw new Error("party is not in attendee-adding mode");
        }

        const i = this.tmpAttendees.findIndex(pub => pub.equals(attendee));
        if (i >= 0) {
            this.tmpAttendees.splice(i, 1);
        }

        return this.tmpAttendees.length;
    }

    /**
     * Start the party
     * 
     * @param signers The list of signers for the transaction
     * @returns a promise that resolves with the state of the party
     */
    async activateBarrier(signers: Signer[]): Promise<number> {
        if (this.popPartyStruct.state !== PopPartyInstance.PreBarrier) {
            throw new Error("barrier point has already been passed");
        }

        const instr = Instruction.createInvoke(
            this.instance.id,
            PopPartyInstance.contractID,
            "barrier",
            [],
        );

        const ctx = new ClientTransaction({ instructions: [instr] });
        await ctx.updateCounters(this.rpc, signers);
        ctx.signWith(signers);

        await this.bc.sendTransactionAndWait(ctx);
        await this.update();

        return this.popPartyStruct.state;
    }

    /**
     * Finalize the party
     * 
     * @param signers The list of signers for the transaction
     * @returns a promise that resolves with the state of the party
     */
    async finalize(signers: Signer[]): Promise<number> {
        if (this.popPartyStruct.state !== PopPartyInstance.Scanning) {
            throw new Error("party did not pass barrier-point yet");
        }

        this.popPartyStruct.updateAttendes(this.tmpAttendees);

        const instr = Instruction.createInvoke(
            this.instance.id,
            PopPartyInstance.contractID,
            "finalize",
            [new Argument({ name: "attendees", value: this.popPartyStruct.attendees.toBytes() })],
        );

        const ctx = new ClientTransaction({ instructions: [instr] });
        await ctx.updateCounters(this.rpc, signers);
        ctx.signWith(signers);

        await this.bc.sendTransactionAndWait(ctx);
        await this.update();

        return this.popPartyStruct.state;
    }

    /**
     * Update the party data
     * @returns a promise that resolves with an updaed instance
     */
    async update(): Promise<PopPartyInstance> {
        this.instance = await Instance.fromByzCoin(this.rpc, this.instance.id);
        this.popPartyStruct = PopPartyStruct.decode(this.instance.data);

        if (this.popPartyStruct.state === PopPartyInstance.Scanning &&
            this.tmpAttendees.length === 0) {
            this.tmpAttendees = await this.fetchOrgKeys();
        }

        return this;
    }

    /**
     * Mine coins for a person using a coin instance ID
     * 
     * @param secret The secret key of the miner
     * @param coinID The coin instance ID of the miner
     */
    async mine(secret: Scalar, coinID?: Buffer): Promise<void> {
        if (this.popPartyStruct.state != PopPartyInstance.Finalized) {
            throw new Error("cannot mine on a non-finalized party");
        }

        const keys = this.popPartyStruct.attendees.publics;
        const lrs = await anon.sign(Buffer.from("mine"), keys, secret, this.instance.id);
        const args = [
            new Argument({ name: "lrs", value: lrs.encode() }),
            new Argument({ name: "coinIID", value: coinID })
        ];

        const instr = Instruction.createInvoke(
            this.instance.id,
            PopPartyInstance.contractID,
            "mine",
            args,
        );

        // the transaction is not signed but there is a counter-measure against
        // replay attacks server-side
        const ctx = new ClientTransaction({ instructions: [instr] });

        await this.bc.sendTransactionAndWait(ctx);
        await this.update();
    }

    private async fetchOrgKeys(): Promise<Point[]> {
        let piDarc = await DarcInstance.fromByzcoin(this.bc, this.instance.darcID);
        let exprOrgs = piDarc.getDarc().rules.list.find(l => l.action == "invoke:popParty.finalize").expr;
        let orgDarcs = exprOrgs.toString().split(" | ");
        let orgPers: Point[] = [];
        for (let i = 0; i < orgDarcs.length; i++) {
            // Remove leading "darc:" from expression
            const orgDarc = orgDarcs[i].substr(5);
            const orgCred = SpawnerInstance.credentialIID(Buffer.from(orgDarc, 'hex'));
            Log.lvl2("Searching personhood-identity of organizer", orgDarc, orgCred);
            const cred = await CredentialInstance.fromByzcoin(this.bc, orgCred);
            const credPers = cred.getAttribute("personhood", "ed25519");
            if (!credPers) {
                throw new Error("found organizer without personhood credential");
            }

            const pub = PointFactory.fromProto(credPers);
            orgPers.push(pub);
        }

        return orgPers;
    }

    /**
     * Get a pop party from byzcoin
     * 
     * @param bc    The RPC to use
     * @param iid   The instance ID of the party
     * @returns a promise that resolves with the party instance
     */
    public static async fromByzcoin(bc: ByzCoinRPC, iid: Buffer): Promise<PopPartyInstance> {
        return new PopPartyInstance(bc, await Instance.fromByzCoin(bc, iid));
    }
}

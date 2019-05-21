import { IIdentity } from "../../darc";
import Darc from "../../darc/darc";
import Signer from "../../darc/signer";
import Log from "../../log";
import ByzCoinRPC from "../byzcoin-rpc";
import ClientTransaction, { Argument, Instruction } from "../client-transaction";
import Instance, { InstanceID } from "../instance";

export default class DarcInstance extends Instance {
    static readonly contractID = "darc";
    static readonly commandEvolve = "evolve";
    static readonly commandEvolveUnrestricted = "evolve_unrestricted";
    static readonly argumentDarc = "darc";
    static readonly ruleEvolve = "invoke:" + DarcInstance.argumentDarc + "." + DarcInstance.commandEvolve;
    static readonly ruleEvolveUnrestricted = "invoke:" + DarcInstance.argumentDarc + "." +
        DarcInstance.commandEvolveUnrestricted;

    /**
     * Initializes using an existing coinInstance from ByzCoin
     *
     * @param bc a working ByzCoin instance
     * @param iid the instance id of the darc-instance
     * @param waitMatch how many times to wait for a match - useful if its called just after an addTransactionAndWait.
     * @param interval how long to wait between two attempts in waitMatch.
     * @returns a promise that resolves with the darc instance
     */
    static async fromByzcoin(bc: ByzCoinRPC, iid: InstanceID, waitMatch: number = 0, interval: number = 1000):
        Promise<DarcInstance> {
        return new DarcInstance(bc, await Instance.fromByzcoin(bc, iid, waitMatch, interval));
    }

    /**
     * spawn creates a new darc, given a darcID.
     *
     * @param rpc a working ByzCoin instance
     * @param darcID a darc that has the right to spawn new darcs
     * @param signers fulfilling the `spawn:darc` rule of the darc pointed to by darcID
     * @param newD the new darc to spawn
     */
    static async spawn(rpc: ByzCoinRPC,
                       darcID: InstanceID,
                       signers: Signer[],
                       newD: Darc): Promise<DarcInstance> {
        const di = await DarcInstance.fromByzcoin(rpc, darcID);
        return di.spawnDarcAndWait(newD, signers, 10);
    }

    /**
     * create returns a DarcInstance, given a ByzCoin and a darc. The instance must already exist on
     * ByzCoin. This method does not verify if it does or not.
     *
     * @param rpc a working ByzCoin instance
     * @param d the darc
     */
    static create(rpc: ByzCoinRPC,
                  d: Darc): DarcInstance {
        return new DarcInstance(rpc, new Instance({
            contractID: DarcInstance.contractID,
            darcID: d.getBaseID(),
            data: d.toBytes(),
            id: d.getBaseID(),
        }));
    }

    private _darc: Darc;

    /**
     * Returns a copy of the darc.
     */
    get darc(): Darc {
        return this._darc.copy();
    }

    constructor(private rpc: ByzCoinRPC, inst: Instance) {
        super(inst);
        if (inst.contractID.toString() !== DarcInstance.contractID) {
            throw new Error(`mismatch contract name: ${inst.contractID} vs ${DarcInstance.contractID}`);
        }

        this._darc = Darc.decode(inst.data);
    }

    /**
     * Update the data of this instance
     *
     * @return a promise that resolves once the data is up-to-date
     */
    async update(): Promise<DarcInstance> {
        const proof = await this.rpc.getProof(this._darc.getBaseID());
        const inst = await proof.getVerifiedInstance(this.rpc.getGenesis().computeHash(), DarcInstance.contractID);
        this._darc = Darc.decode(inst.data);

        return this;
    }

    /**
     * Searches for the rule that corresponds to the Darc.ruleSign action. If that rule
     * does not exist, it returns an error.
     */
    getSignerExpression(): Buffer {
        for (const rule of this._darc.rules.list) {
            if (rule.action === Darc.ruleSign) {
                return rule.expr;
            }
        }
        throw new Error("This darc doesn't have a sign expression");
    }

    /**
     * Returns all darcs that are stored in the signer expression. It leaves out any
     * other element of the expression.
     */
    getSignerDarcIDs(): InstanceID[] {
        const expr = this.getSignerExpression().toString();
        if (expr.match(/\(&/)) {
            throw new Error('Don\'t know what to do with "(" or "&" in expression');
        }
        const ret: InstanceID[] = [];
        expr.split("|").forEach((e) => {
            const exp = e.trim();
            if (exp.startsWith("darc:")) {
                ret.push(Buffer.from(exp.slice(5), "hex"));
            } else {
                Log.warn("Non-darc expression in signer:", exp);
            }
        });
        return ret;
    }

    /**
     * Request to evolve the existing darc using the new darc and wait for
     * the block inclusion
     *
     * @param newDarc The new darc
     * @param signers Signers for the counters
     * @param wait Number of blocks to wait for
     * @returns a promise that resolves with the new darc instance
     */
    async evolveDarcAndWait(newDarc: Darc, signers: Signer[], wait: number,
                            unrestricted: boolean = false): Promise<DarcInstance> {
        if (!newDarc.getBaseID().equals(this._darc.getBaseID())) {
            throw new Error("not the same base id for the darc");
        }
        if (newDarc.version.compare(this._darc.version.add(1)) !== 0) {
            throw new Error("not the right version");
        }
        if (!newDarc.prevID.equals(this._darc.id)) {
            throw new Error("doesn't point to the previous darc");
        }
        const args = [new Argument({name: DarcInstance.argumentDarc,
            value: Buffer.from(Darc.encode(newDarc).finish())})];
        const cmd = unrestricted ? DarcInstance.commandEvolveUnrestricted : DarcInstance.commandEvolve;
        const instr = Instruction.createInvoke(this._darc.getBaseID(),
            DarcInstance.contractID, cmd, args);

        const ctx = new ClientTransaction({instructions: [instr]});
        await ctx.updateCounters(this.rpc, [signers]);
        ctx.signWith([signers]);

        await this.rpc.sendTransactionAndWait(ctx, wait);

        return this.update();
    }

    /**
     * Request to spawn an instance and wait for the inclusion
     *
     * @param d             The darc to spawn
     * @param signers       Signers for the counters
     * @param wait          Number of blocks to wait for
     * @returns a promise that resolves with the new darc instance
     */
    async spawnDarcAndWait(d: Darc, signers: Signer[], wait: number = 0): Promise<DarcInstance> {
        await this.spawnInstanceAndWait(DarcInstance.contractID,
            [new Argument({
                name: DarcInstance.argumentDarc,
                value: Buffer.from(Darc.encode(d).finish()),
            })], signers, wait);
        return DarcInstance.fromByzcoin(this.rpc, d.getBaseID());
    }

    /**
     * Request to spawn an instance of any contract and wait
     *
     * @param contractID    Contract name of the new instance
     * @param signers       Signers for the counters
     * @param wait          Number of blocks to wait for
     * @returns a promise that resolves with the instanceID of the new instance, which is only valid if the
     *          contract.spawn uses DeriveID.
     */
    async spawnInstanceAndWait(contractID: string, args: Argument[], signers: Signer[], wait: number = 0):
        Promise<InstanceID> {
        const instr = Instruction.createSpawn(this._darc.getBaseID(), DarcInstance.contractID, args);

        const ctx = new ClientTransaction({instructions: [instr]});
        await ctx.updateCounters(this.rpc, [signers]);
        ctx.signWith([signers]);

        await this.rpc.sendTransactionAndWait(ctx, wait);

        return ctx.instructions[0].deriveId();
    }

    /**
     * Checks whether the given rule can be matched by a multi-signature created by all
     * signers. If the rule doesn't exist, this method silently returns 'false'.
     * Currently only Rules.OR are supported. A Rules.AND or "(" will return an error.
     * Currently only 1 signer is supported.
     *
     * @param action the action to match
     * @param signers all supposed signers for this action.
     */
    async ruleMatch(action: string, signers: IIdentity[]): Promise<boolean> {
        const ids = await this._darc.ruleMatch(action, signers, async (id: Buffer) => {
            const di = await DarcInstance.fromByzcoin(this.rpc, id);
            return di._darc;
        });
        return ids.length > 0;
    }
}

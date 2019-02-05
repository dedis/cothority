import { createHash } from "crypto";
import Long from "long";
import { Message } from "protobufjs";
import { Point } from "@dedis/kyber";
import ByzCoinRPC from "../byzcoin-rpc";
import { InstanceID } from "../instance";
import CoinInstance, { Coin } from "./coin-instance";
import Signer from "../../darc/signer";
import DarcInstance from "./darc-instance";
import Log from "../../log";
import ClientTransaction, { Instruction, Argument } from "../client-transaction";
import CredentialInstance, { CredentialStruct } from "./credentials-instance";
import { PopDesc } from "./pop-party/proto";
import { PopPartyInstance } from "./pop-party/pop-party-instance";
import RoPaSciInstance, { RoPaSciStruct } from "./ro-pa-sci-instance";
import Darc from "../../darc/darc";
import IdentityEd25519 from "../../darc/identity-ed25519";
import Rules from "../../darc/rules";
import { registerMessage } from "../../protobuf";
import IdentityDarc from "../../darc/identity-darc";

export const SpawnerCoin = Buffer.alloc(32, 0);
SpawnerCoin.write('SpawnerCoin');

export default class SpawnerInstance {
    static readonly contractID = "spawner";

    private rpc: ByzCoinRPC;
    readonly iid: InstanceID;
    private struct: SpawnerStruct;

    /**
     * Creates a new SpawnerInstance
     * @param bc        The ByzCoinRPC instance
     * @param iid       The instance ID
     * @param spawner   Parameters for the spawner: costs and names
     */
    constructor(bc: ByzCoinRPC, iid: InstanceID, spawner: SpawnerStruct) {
        this.rpc = bc;
        this.iid = iid;
        this.struct = spawner
    }

    /**
     * Get the total cost required to sign up
     * 
     * @returns the cost
     */
    get signupCost(): Long {
        return this.struct.costCoin.value
            .add(this.struct.costDarc.value)
            .add(this.struct.costCredential.value);
    }

    /**
     * Update the data of this instance
     *
     * @returns a promise that resolves once the data is up-to-date
     */
    async update(): Promise<SpawnerInstance> {
        let proof = await this.rpc.getProof(this.iid);
        this.struct = SpawnerStruct.decode(proof.value);
        return this;
    }

    /**
     * Create a darc for a user
     * 
     * @param coin      The coin instance to take coins from
     * @param signers   The signers for the transaction
     * @param pubKey    public key of the user
     * @param alias     Name of the user
     * @returns a promise that resolves with the new darc instance
     */
    async createUserDarc(coin: CoinInstance, signers: Signer[], pubKey: Point, alias: string): Promise<DarcInstance> {
        const d = SpawnerInstance.prepareUserDarc(pubKey, alias);
        try {
            const darc = await DarcInstance.fromByzcoin(this.rpc, d.baseID);
            Log.lvl2("this darc is already registerd");
            return darc;
        } catch (e) {
            // darc already exists
        }

        const ctx = new ClientTransaction({
            instructions: [
                Instruction.createInvoke(
                    coin.id,
                    CoinInstance.contractID,
                    "fetch",
                    [new Argument({ name: "coins", value: Buffer.from(this.struct.costDarc.value.toBytesLE()) })],
                ),
                Instruction.createSpawn(
                    this.iid,
                    DarcInstance.contractID,
                    [new Argument({ name: "darc", value: d.toBytes() })],
                ),
            ]
        });
        await ctx.updateCounters(this.rpc, signers);
        ctx.signWith(signers);

        await this.rpc.sendTransactionAndWait(ctx);

        return DarcInstance.fromByzcoin(this.rpc, d.baseID);
    }

    /**
     * Create a coin instance for a given darc
     * 
     * @param coin      The coin instance to take the coins from
     * @param signers   The signers for the transaction
     * @param darcID    The darc instance ID
     * @param balance   The starting balance
     * @returns a promise that resolves with the new coin instance
     */
    async createCoin(coin: CoinInstance, signers: Signer[], darcID: Buffer, balance?: Long): Promise<CoinInstance> {
        try {
            const ci = await CoinInstance.fromByzcoin(this.rpc, SpawnerInstance.coinIID(darcID));
            Log.lvl2("this coin is already registered");
            return ci;
        } catch (e) {
            // doesn't exist
        }

        balance = balance || Long.fromNumber(0);
        const valueBuf = this.struct.costCoin.value.add(balance).toBytesLE();
        const ctx = new ClientTransaction({
            instructions: [
                Instruction.createInvoke(
                    coin.id,
                    CoinInstance.contractID,
                    "fetch",
                    [new Argument({ name: "coins", value: Buffer.from(valueBuf) })],
                ),
                Instruction.createSpawn(
                    this.iid,
                    CoinInstance.contractID,
                    [
                        new Argument({ name: "coinName", value: SpawnerCoin }),
                        new Argument({ name: "darcID", value: darcID }),
                    ],
                )
            ]
        });
        await ctx.updateCounters(this.rpc, signers);
        ctx.signWith(signers);

        await this.rpc.sendTransactionAndWait(ctx);

        return CoinInstance.fromByzcoin(this.rpc, SpawnerInstance.coinIID(darcID));
    }

    /**
     * Create a credential instance for the given darc
     * 
     * @param coin      The coin instance to take coins from
     * @param signers   The signers for the transaction
     * @param darcID    The darc instance ID
     * @param cred      The starting credentials
     * @returns a promise that resolves with the new credential instance
     */
    async createCredential(coin: CoinInstance, signers: Signer[], darcID: Buffer, cred: CredentialStruct): Promise<CredentialInstance> {
        try {
            const cred = await CredentialInstance.fromByzcoin(this.rpc, SpawnerInstance.credentialIID(darcID));
            Log.lvl2("this credential is already registerd");
            return cred;
        } catch (e) {
            // credential doesn't exist
        }

        const valueBuf = this.struct.costCredential.value.toBytesLE();
        const ctx = new ClientTransaction({
            instructions: [
                Instruction.createInvoke(
                    coin.id,
                    CoinInstance.contractID,
                    "fetch",
                    [new Argument({ name: "coins", value: Buffer.from(valueBuf) })],
                ),
                Instruction.createSpawn(
                    this.iid,
                    CredentialInstance.contractID,
                    [
                        new Argument({ name: "darcID", value: darcID }),
                        new Argument({ name: "credential", value: cred.toBytes() }),
                    ],
                ),
            ],
        });
        await ctx.updateCounters(this.rpc, signers);
        ctx.signWith(signers);

        await this.rpc.sendTransactionAndWait(ctx);

        return CredentialInstance.fromByzcoin(this.rpc, SpawnerInstance.credentialIID(darcID));
    }

    /**
     * Create a PoP party
     * 
     * @param coin The coin instance to take coins from
     * @param signers The signers for the transaction
     * @param orgs The list fo organisers
     * @param descr The data for the PoP party
     * @param reward The reward of an attendee
     * @returns a promise tha resolves with the new pop party instance
     */
    async createPopParty(params: CreatePopParty): Promise<PopPartyInstance> {
        const { coin, signers, orgs, desc, reward } = params;

        // Verify that all organizers have published their personhood public key
        for (const org of orgs) {
            if (!org.getAttribute('personhood', 'ed25519')) {
                throw new Error(`One of the organisers didn't publish his personhood key`);
            }
        }

        const orgDarcIDs = orgs.map(org => org.darcID);
        const valueBuf = this.struct.costDarc.value.add(this.struct.costParty.value).toBytesLE();
        const orgDarc = SpawnerInstance.preparePartyDarc(orgDarcIDs, "party-darc " + desc.name);
        const ctx = new ClientTransaction({
            instructions: [
                Instruction.createInvoke(
                    coin.id,
                    CoinInstance.contractID,
                    "fetch",
                    [new Argument({ name: "coins", value: Buffer.from(valueBuf) })],
                ),
                Instruction.createSpawn(
                    this.iid,
                    DarcInstance.contractID,
                    [new Argument({ name: "darc", value: orgDarc.toBytes() })],
                ),
                Instruction.createSpawn(
                    this.iid,
                    PopPartyInstance.contractID,
                    [
                        new Argument({ name: "darcID", value: orgDarc.baseID }),
                        new Argument({ name: "description", value: desc.toBytes() }),
                        new Argument({ name: "miningReward", value: Buffer.from(reward.toBytesLE()) }),
                    ],
                )
            ]
        });
        await ctx.updateCounters(this.rpc, signers);
        ctx.signWith(signers);

        await this.rpc.sendTransactionAndWait(ctx);

        return PopPartyInstance.fromByzcoin(this.rpc, ctx.instructions[2].deriveId());
    }

    /**
     * Create a Rock-Paper-Scisors game instance
     * 
     * @param desc      The description of the game
     * @param coin      The coin instance to take coins from
     * @param signers   The list of signers
     * @param stake     The reward for the winner
     * @param choice    The choice of the first player
     * @param fillup    Data that will be hash with the choice
     * @returns a promise that resolves with the new instance
     */
    async createRoPaSci(params: CreateRoPaSci): Promise<RoPaSciInstance> {
        const { desc, coin, signers, stake, choice, fillup } = params;
        
        if (fillup.length != 31){
            throw new Error("need exactly 31 bytes for fillUp");
        }

        const c = new Coin({name: coin.name, value: stake.add(this.struct.costRoPaSci.value) });
        if (coin.value.lessThan(c.value)){
            throw new Error("account balance not high enough for that stake");
        }

        const fph = createHash("sha256");
        fph.update(Buffer.from([choice % 3]));
        fph.update(fillup);
        const rps = new RoPaSciStruct({
            description: desc,
            stake: c,
            firstplayerhash: fph.digest(),
            firstplayer: -1,
            secondplayer: -1,
            secondplayeraccount: null,
        });

        const ctx = new ClientTransaction({
            instructions: [
                Instruction.createInvoke(
                    coin.id,
                    CoinInstance.contractID,
                    "fetch",
                    [new Argument({ name: "coins", value: Buffer.from(c.value.toBytesLE()) })]
                ),
                Instruction.createSpawn(
                    this.iid,
                    RoPaSciInstance.contractID,
                    [new Argument({ name: "struct", value: rps.toBytes() })],
                )
            ],
        });
        await ctx.updateCounters(this.rpc, signers);
        ctx.signWith(signers);

        await this.rpc.sendTransactionAndWait(ctx);

        const rpsi = await RoPaSciInstance.fromByzcoin(this.rpc, ctx.instructions[1].deriveId());
        rpsi.setChoice(choice, fillup);

        return rpsi;
    }

    /**
     * Create a spawner instance
     * 
     * @param bc The ByzCoinRPC to use
     * @param darcID The darc instance ID
     * @param signers The list of signers
     * @param costs The different cost for new instances
     * @param beneficiary The beneficiary of the costs
     */
    static async create(params: CreateSpawner): Promise<SpawnerInstance> {
        const { bc, darcID, signers, costs, beneficiary } = params;

        const args = [
            ...Object.keys(costs).map((k) => {
                const value = new Coin({ name: SpawnerCoin, value: costs[k] }).toBytes();
                return new Argument({ name: k, value });
            }),
            new Argument({ name: 'beneficiary', value: beneficiary }),
        ];

        const inst = Instruction.createSpawn(darcID, this.contractID, args);
        const ctx = new ClientTransaction({ instructions: [inst] });
        await ctx.updateCounters(bc, signers);
        ctx.signWith(signers);

        await bc.sendTransactionAndWait(ctx);

        return this.fromByzcoin(bc, inst.deriveId());
    }

    /**
     * Initializes using an existing coinInstance from ByzCoin
     * 
     * @param bc
     * @param iid
     */
    static async fromByzcoin(bc: ByzCoinRPC, iid: InstanceID): Promise<SpawnerInstance> {
        const proof = await bc.getProof(iid);
        if (!proof.exists(iid)) {
            throw new Error('fail to get a matching proof');
        }

        return new SpawnerInstance(bc, iid, SpawnerStruct.decode(proof.value));
    }

    /**
     * Helper to create a user darc
     * 
     * @param pubKey    The user public key
     * @param alias     The user alias
     * @returns the new darc
     */
    static prepareUserDarc(pubKey: Point, alias: string): Darc {
        const id = new IdentityEd25519({ point: pubKey.toProto() });

        const darc = Darc.newDarc([id], [id], Buffer.from(`user ${alias}`));
        darc.addIdentity('invoke:coin.update', id, Rules.AND);
        darc.addIdentity('invoke:coin.fetch', id, Rules.AND);
        darc.addIdentity('invoke:coin.transfer', id, Rules.AND);

        return darc;
    }

    /**
     * Helper to create a PoP party darc
     * 
     * @param darcIDs   Organizers darc instance IDs
     * @param desc      Description of the party
     * @returns the new darc
     */
    static preparePartyDarc(darcIDs: InstanceID[], desc: string): Darc {
        const ids = darcIDs.map(di => new IdentityDarc({ id: di }));
        const darc = Darc.newDarc(ids, ids, Buffer.from(desc));
        ids.forEach((id) => {
            darc.addIdentity('invoke:popParty.barrier', id, Rules.OR);
            darc.addIdentity('invoke:popParty.finalize', id, Rules.OR);
            darc.addIdentity('invoke:popParty.addParty', id, Rules.OR);
        });

        return darc;
    }

    /**
     * Generate the credential instance ID for a given darc ID
     * 
     * @param darcBaseID The base ID of the darc
     * @returns the id as a buffer
     */
    static credentialIID(darcBaseID: Buffer): InstanceID {
        let h = createHash("sha256");
        h.update(Buffer.from("credential"));
        h.update(darcBaseID);
        return h.digest();
    }

    /**
     * Generate the coin instance ID for a given darc ID
     * 
     * @param darcBaseID The base ID of the darc
     * @returns the id as a buffer
     */
    static coinIID(darcBaseID: Buffer): InstanceID {
        let h = createHash("sha256");
        h.update(Buffer.from("coin"));
        h.update(darcBaseID);
        return h.digest();
    }
}

/**
 * Data of a spawner instance
 */
export class SpawnerStruct extends Message<SpawnerStruct> {
    readonly costdarc: Coin;
    readonly costcoin: Coin;
    readonly costcredential: Coin;
    readonly costparty: Coin;
    readonly costropasci: Coin;
    readonly beneficiary: InstanceID;

    get costDarc(): Coin {
        return this.costdarc;
    }

    get costCoin(): Coin {
        return this.costcoin;
    }

    get costCredential(): Coin {
        return this.costcredential;
    }

    get costParty(): Coin {
        return this.costparty;
    }

    get costRoPaSci(): Coin {
        return this.costropasci;
    }
}

/**
 * Fields of the costs of a spawner instance
 */
interface CreateCost {
    [k: string]: Long
    costDarc: Long,
    costCoin: Long,
    costCredential: Long,
    costParty: Long,
}

/**
 * Parameters to create a spawner instance
 */
interface CreateSpawner {
    [k: string]: any
    bc: ByzCoinRPC,
    darcID: InstanceID,
    signers: Signer[],
    costs: CreateCost,
    beneficiary: InstanceID,
}

/**
 * Parameters to create a rock-paper-scisors game
 */
interface CreateRoPaSci {
    [k: string]: any
    desc: string,
    coin: CoinInstance,
    signers: Signer[],
    stake: Long,
    choice: number,
    fillup: Buffer,
}

/**
 * Parameters to create a pop party
 */
interface CreatePopParty {
    [k: string]: any
    coin: CoinInstance,
    signers: Signer[],
    orgs: CredentialInstance[],
    desc: PopDesc,
    reward: Long,
}

registerMessage('personhood.SpawnerStruct', SpawnerStruct);

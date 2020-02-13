import { createHash, randomBytes } from "crypto-browserify";
import Long from "long";
import { Message, Properties } from "protobufjs/light";
import ByzCoinRPC from "../byzcoin/byzcoin-rpc";
import ClientTransaction, { Argument, Instruction } from "../byzcoin/client-transaction";
import CoinInstance, { Coin } from "../byzcoin/contracts/coin-instance";
import DarcInstance from "../byzcoin/contracts/darc-instance";
import ValueInstance from "../byzcoin/contracts/value-instance";
import Instance, { InstanceID } from "../byzcoin/instance";
import { CalypsoReadInstance } from "../calypso";
import { CalypsoWriteInstance, Write } from "../calypso/calypso-instance";
import { LongTermSecret } from "../calypso/calypso-rpc";
import { IIdentity } from "../darc";
import Darc from "../darc/darc";
import { Rule } from "../darc/rules";
import Signer from "../darc/signer";
import ISigner from "../darc/signer";
import Log from "../log";
import { registerMessage } from "../protobuf";
import CredentialInstance from "./credentials-instance";
import CredentialsInstance, { CredentialStruct } from "./credentials-instance";
import { PopPartyInstance } from "./pop-party-instance";
import { PopDesc } from "./proto";
import RoPaSciInstance, { RoPaSciStruct } from "./ro-pa-sci-instance";

export const SPAWNER_COIN = Buffer.alloc(32, 0);
SPAWNER_COIN.write("SpawnerCoin");

export default class SpawnerInstance extends Instance {

    get costs(): SpawnerStruct {
        return new SpawnerStruct(this.struct);
    }

    static readonly contractID = "spawner";
    static readonly argumentCredential = "credential";
    static readonly argumentCredID = "credID";
    static readonly argumentDarc = "darc";
    static readonly argumentDarcID = "darcID";
    static readonly argumentCoinID = "coinID";
    static readonly argumentCoinName = "coinName";

    /**
     * Spawn a spawner instance. It takes either an ICreateSpawner as single argument, or all the arguments
     * separated.
     *
     * @param params The ByzCoinRPC to use or an ICreateSpawner
     * @param darcID The darc instance ID
     * @param signers The list of signers
     * @param costs The different cost for new instances
     * @param beneficiary The beneficiary of the costs
     */
    static async spawn(params: ICreateSpawner | ByzCoinRPC, darcID?: InstanceID, signers?: Signer[],
                       costs?: ICreateCost,
                       beneficiary?: InstanceID): Promise<SpawnerInstance> {
        let bc: ByzCoinRPC;
        if (params instanceof ByzCoinRPC) {
            bc = params as ByzCoinRPC;
        } else {
            ({bc, darcID, signers, costs, beneficiary} = params as ICreateSpawner);
        }

        const args = [
            ...Object.keys(costs).map((k: string) => {
                const value = new Coin({name: SPAWNER_COIN, value: costs[k]}).toBytes();
                return new Argument({name: k, value});
            }),
            new Argument({name: "beneficiary", value: beneficiary}),
        ];

        const inst = Instruction.createSpawn(darcID, this.contractID, args);
        const ctx = ClientTransaction.make(bc.getProtocolVersion(), inst);
        await ctx.updateCountersAndSign(bc, [signers]);
        await bc.sendTransactionAndWait(ctx);

        return this.fromByzcoin(bc, ctx.instructions[0].deriveId(), 1);
    }

    /**
     * Initializes using an existing coinInstance from ByzCoin
     *
     * @param bc an initialized byzcoin RPC instance
     * @param iid the instance-ID of the spawn-instance
     * @param waitMatch how many times to wait for a match - useful if its called just after an addTransactionAndWait.
     * @param interval how long to wait between two attempts in waitMatch.
     */
    static async fromByzcoin(bc: ByzCoinRPC, iid: InstanceID, waitMatch: number = 0, interval: number = 1000):
        Promise<SpawnerInstance> {
        return new SpawnerInstance(bc, await Instance.fromByzcoin(bc, iid, waitMatch, interval));
    }
    private struct: SpawnerStruct;

    /**
     * Creates a new SpawnerInstance
     * @param rpc        The ByzCoinRPC instance
     * @param inst   A valid spawner instance
     */
    constructor(private rpc: ByzCoinRPC, inst: Instance) {
        super(inst);
        if (inst.contractID.toString() !== SpawnerInstance.contractID) {
            throw new Error(`mismatch contract name: ${inst.contractID} vs ${SpawnerInstance.contractID}`);
        }

        this.struct = SpawnerStruct.decode(inst.data);
    }

    /**
     * Update the data of this instance
     *
     * @returns a promise that resolves once the data is up-to-date
     */
    async update(): Promise<SpawnerInstance> {
        const proof = await this.rpc.getProofFromLatest(this.id);
        this.struct = SpawnerStruct.decode(proof.value);
        return this;
    }

    /**
     * Create the instructions necessary to spawn one or more darcs. This is separated from the
     * spanDarcs method itself, so that a caller can create a bigger ClientTransaction with
     * multiple sets of instructions inside.
     *
     * @param coin where to take the coins from
     * @param darcs the darcs to create
     */
    spawnDarcInstructions(coin: CoinInstance, ...darcs: Darc[]): Instruction[] {
        const cost = this.struct.costDarc.value.mul(darcs.length);
        const ret: Instruction[] = [
            Instruction.createInvoke(
                coin.id,
                CoinInstance.contractID,
                CoinInstance.commandFetch,
                [new Argument({name: CoinInstance.argumentCoins, value: Buffer.from(cost.toBytesLE())})],
            ),
            ...darcs.map((darc) =>
                Instruction.createSpawn(
                    this.id,
                    DarcInstance.contractID,
                    [new Argument({name: SpawnerInstance.argumentDarc, value: darc.toBytes()})],
                )),
        ];
        return ret;
    }

    /**
     * Spawns one or more darc and returns an array of all the spawned darcs.
     *
     * @param coin      The coin instance to take coins from
     * @param signers   The signers for the transaction
     * @param darcs... All the darcs to spawn using the spawner. The coin instance must have enough
     * coins to pay for all darcs.
     * @returns a promise that resolves with the new array of the instantiated darc instances
     */
    async spawnDarcs(coin: CoinInstance, signers: Signer[], ...darcs: Darc[]): Promise<DarcInstance[]> {
        const ctx = ClientTransaction.make(
            this.rpc.getProtocolVersion(),
            ...this.spawnDarcInstructions(coin, ...darcs),
        );
        await ctx.updateCountersAndSign(this.rpc, [signers]);

        await this.rpc.sendTransactionAndWait(ctx);

        const dis = [];
        for (const da of darcs) {
            try {
                // Because this call is directly after the sendTransactionAndWait, the new block might not be
                // applied to all nodes yet.
                dis.push(await DarcInstance.fromByzcoin(this.rpc, da.getBaseID(), 2, 1000));
                break;
            } catch (e) {
                Log.warn("couldn't get proof - perhaps still updating?");
            }
        }
        return dis;
    }

    /**
     * Creates all the necessary instruction to create a new coin - either with a 0 balance, or with
     * a given balance by the caller.
     *
     * @param coin where to take the coins to create the instance
     * @param darcID the responsible darc for the new coin
     * @param coinID the type of coin - must be the same as the `coin` in case of balance > 0
     * @param balance how many coins to transfer to the new coin
     */
    spawnCoinInstructions(coin: CoinInstance, darcID: InstanceID, coinID: Buffer, balance?: Long): Instruction[] {
        balance = balance || Long.fromNumber(0);
        const valueBuf = this.struct.costCoin.value.add(balance).toBytesLE();
        return [
            Instruction.createInvoke(
                coin.id,
                CoinInstance.contractID,
                CoinInstance.commandFetch,
                [new Argument({name: CoinInstance.argumentCoins, value: Buffer.from(valueBuf)})],
            ),
            Instruction.createSpawn(
                this.id,
                CoinInstance.contractID,
                [
                    new Argument({name: SpawnerInstance.argumentCoinName, value: SPAWNER_COIN}),
                    new Argument({name: SpawnerInstance.argumentCoinID, value: coinID}),
                    new Argument({name: SpawnerInstance.argumentDarcID, value: darcID}),
                ],
            ),
        ];
    }

    /**
     * Create a coin instance for a given darc
     *
     * @param coin      The coin instance to take the coins from
     * @param signers   The signers for the transaction
     * @param darcID    The darc responsible for this coin
     * @param coinID    The instance-ID for the coin - will be calculated as sha256("coin" | coinID)
     * @param balance   The starting balance
     * @returns a promise that resolves with the new coin instance
     */
    async spawnCoin(coin: CoinInstance, signers: Signer[], darcID: InstanceID, coinID: Buffer, balance?: Long):
        Promise<CoinInstance> {
        const ctx = ClientTransaction.make(
            this.rpc.getProtocolVersion(),
            ...this.spawnCoinInstructions(coin, darcID, coinID, balance),
        );
        await ctx.updateCountersAndSign(this.rpc, [signers, []]);
        await this.rpc.sendTransactionAndWait(ctx);

        return CoinInstance.fromByzcoin(this.rpc, CoinInstance.coinIID(coinID), 2);
    }

    /**
     * Creates the instructions necessary to create a credential. This is separated from the spawnCredential
     * method, so that a caller can get the instructions separated and then put all the instructions
     * together in a big ClientTransaction.
     *
     * @param coin coin-instance to pay for the credential
     * @param darcID responsible darc for the credential
     * @param cred the credential structure
     * @param credID if given, used to calculate the iid of the credential, else the darcID will be used
     */
    spawnCredentialInstruction(coin: CoinInstance, darcID: Buffer, cred: CredentialStruct, credID: Buffer = null):
        Instruction[] {
        const valueBuf = this.struct.costCredential.value.toBytesLE();
        const credArgs = [
            new Argument({name: SpawnerInstance.argumentDarcID, value: darcID}),
            new Argument({name: SpawnerInstance.argumentCredential, value: cred.toBytes()}),
        ];
        if (credID) {
            credArgs.push(new Argument({name: SpawnerInstance.argumentCredID, value: credID}));
        }
        return [
            Instruction.createInvoke(
                coin.id,
                CoinInstance.contractID,
                CoinInstance.commandFetch,
                [new Argument({name: CoinInstance.argumentCoins, value: Buffer.from(valueBuf)})],
            ),
            Instruction.createSpawn(
                this.id,
                CredentialInstance.contractID,
                credArgs,
            ),
        ];
    }

    /**
     * Create a credential instance for the given darc
     *
     * @param coin      The coin instance to take coins from
     * @param signers   The signers for the transaction
     * @param darcID    The darc instance ID
     * @param cred      The starting credentials
     * @param credID    The instance-ID for this credential - will be calculated as sha256("credential" | credID)
     * @returns a promise that resolves with the new credential instance
     */
    async spawnCredential(
        coin: CoinInstance,
        signers: ISigner[],
        darcID: Buffer,
        cred: CredentialStruct,
        credID: Buffer = null,
    ): Promise<CredentialsInstance> {
        const ctx = ClientTransaction.make(
            this.rpc.getProtocolVersion(),
            ...this.spawnCredentialInstruction(coin, darcID, cred, credID),
        );
        await ctx.updateCountersAndSign(this.rpc, [signers, []]);
        await this.rpc.sendTransactionAndWait(ctx);

        const finalCredID = CredentialInstance.credentialIID(credID || darcID);
        return CredentialInstance.fromByzcoin(this.rpc, finalCredID, 2);
    }

    /**
     * Create a PoP party
     *
     * @param params structure of {coin, signers, orgs, desc, reward}
     * @returns a promise tha resolves with the new pop party instance
     */
    async spawnPopParty(params: ICreatePopParty): Promise<PopPartyInstance> {
        const {coin, signers, orgs, desc, reward} = params;

        const valueBuf = this.struct.costDarc.value.add(this.struct.costParty.value).toBytesLE();
        const orgDarc = PopPartyInstance.preparePartyDarc(orgs, "party-darc " + desc.name);
        const ctx = ClientTransaction.make(
            this.rpc.getProtocolVersion(),
            Instruction.createInvoke(
                coin.id,
                CoinInstance.contractID,
                CoinInstance.commandFetch,
                [new Argument({name: CoinInstance.argumentCoins, value: Buffer.from(valueBuf)})],
            ),
            Instruction.createSpawn(
                this.id,
                DarcInstance.contractID,
                [new Argument({name: SpawnerInstance.argumentDarc, value: orgDarc.toBytes()})],
            ),
            Instruction.createSpawn(
                this.id,
                PopPartyInstance.contractID,
                [
                    new Argument({name: "darcID", value: orgDarc.getBaseID()}),
                    new Argument({name: "description", value: desc.toBytes()}),
                    new Argument({name: "miningReward", value: Buffer.from(reward.toBytesLE())}),
                ],
            ),
        );
        await ctx.updateCountersAndSign(this.rpc, [signers, [], []]);

        await this.rpc.sendTransactionAndWait(ctx);

        return PopPartyInstance.fromByzcoin(this.rpc, ctx.instructions[2].deriveId(), 2);
    }

    /**
     * Create a Rock-Paper-scissors game instance
     *
     * @param params structure of {desc, coin, signers, stake, choice, fillup}
     * @returns a promise that resolves with the new instance
     */
    async spawnRoPaSci(params: ICreateRoPaSci): Promise<RoPaSciInstance> {
        const {desc, coin, signers, stake, choice, fillup, calypso} = params;

        if (fillup.length !== 31) {
            throw new Error("need exactly 31 bytes for fillUp");
        }

        const c = new Coin({name: coin.name, value: stake.add(this.struct.costRoPaSci.value)});
        if (coin.value.lessThan(c.value)) {
            throw new Error("account balance not high enough for that stake");
        }

        const preHash = Buffer.allocUnsafe(32);
        preHash.writeInt8(choice % 3, 0);
        fillup.copy(preHash, 1);
        const fph = createHash("sha256");
        fph.update(preHash);
        const rps = new RoPaSciStruct({
            description: desc,
            firstPlayer: -1,
            firstPlayerAccount: calypso !== undefined ? coin.id : undefined,
            firstPlayerHash: fph.digest(),
            secondPlayer: -1,
            secondPlayerAccount: Buffer.alloc(32),
            stake: c,
        });

        const rpsArgs = [new Argument({name: "struct", value: rps.toBytes()})];
        if (calypso !== undefined) {
            const wcH = createHash("sha256");
            wcH.update(rps.firstPlayerHash);
            const writeCommit = wcH.digest();
            const w = await Write.createWrite(calypso.id, writeCommit, calypso.X, preHash.slice(0, 28));
            const writeBuf = Write.encode(w).finish();
            rpsArgs.push(new Argument({name: "secret", value: Buffer.from(writeBuf)}));
        }
        const ctx = ClientTransaction.make(
            this.rpc.getProtocolVersion(),
            Instruction.createInvoke(
                coin.id,
                CoinInstance.contractID,
                CoinInstance.commandFetch,
                [new Argument({name: CoinInstance.argumentCoins, value: Buffer.from(c.value.toBytesLE())})],
            ),
            Instruction.createSpawn(
                this.id,
                RoPaSciInstance.contractID,
                rpsArgs,
            ),
        );
        await ctx.updateCountersAndSign(this.rpc, [signers, []]);

        await this.rpc.sendTransactionAndWait(ctx);

        const rpsi = await RoPaSciInstance.fromByzcoin(this.rpc, ctx.instructions[1].deriveId(), 2);
        rpsi.setChoice(choice, fillup);

        return rpsi;
    }

    /**
     * Creates a new calypso write instance for a given key. The key will be encrypted under the
     * aggregated public key. This method creates both the darc that will be protecting the
     * calypsoWrite, as well as the calypsoWrite instance.
     *
     * @param coinInst this coin instance will pay for spawning the calypso write
     * @param signers allow the `invoke:coin.fetch` call on the coinInstance
     * @param lts the id of the long-term-secret that will re-encrypt the key
     * @param key the symmetric key that will be stored encrypted on-chain - not more than 31 bytes.
     * @param ident allowed to re-encrypt the symmetric key
     * @param data additionl data that will be stored AS-IS on chain! So it must be either encrypted using
     * the symmetric 'key', or meta data that is public.
     */
    async spawnCalypsoWrite(coinInst: CoinInstance, signers: Signer[], lts: LongTermSecret, key: Buffer,
                            ident: IIdentity[], data?: Buffer):
        Promise<CalypsoWriteInstance> {

        if (coinInst.value.lessThan(this.struct.costDarc.value.add(this.struct.costCWrite.value))) {
            throw new Error("account balance not high enough for spawning a darc + calypso writer");
        }

        const cwDarc = Darc.createBasic([ident[0]], [ident[0]],
            Buffer.from("calypso write protection " + randomBytes(8).toString("hex")),
            ["spawn:" + CalypsoReadInstance.contractID]);
        ident.slice(1).forEach((id) => cwDarc.rules.appendToRule("spawn:calypsoRead", id, Rule.OR));
        const d = await this.spawnDarcs(coinInst, signers, cwDarc);

        const write = await Write.createWrite(lts.id, d[0].id, lts.X, key);
        write.cost = this.struct.costCRead;
        if (data) {
            write.data = data;
        }

        const ctx = ClientTransaction.make(
            this.rpc.getProtocolVersion(),
            Instruction.createInvoke(coinInst.id, CoinInstance.contractID, CoinInstance.commandFetch, [
                new Argument({
                    name: CoinInstance.argumentCoins,
                    value: Buffer.from(this.struct.costCWrite.value.toBytesLE()),
                }),
            ]),
            Instruction.createSpawn(this.id, CalypsoWriteInstance.contractID, [
                new Argument({
                    name: CalypsoWriteInstance.argumentWrite,
                    value: Buffer.from(Write.encode(write).finish()),
                }),
                new Argument({name: "darcID", value: d[0].id}),
            ]),
        );
        await ctx.updateCountersAndSign(this.rpc, [signers, []]);
        await this.rpc.sendTransactionAndWait(ctx);

        return CalypsoWriteInstance.fromByzcoin(this.rpc, ctx.instructions[1].deriveId(), 2);
    }

    /**
     * Creates all the necessary instruction to create a new value - either with a 0 balance, or with
     * a given balance by the caller.
     *
     * @param coin where to take the coins to create the instance
     * @param darcID the responsible darc for the new coin
     * @param value the value to store in the instance
     */
    spawnValueInstructions(coin: CoinInstance, darcID: InstanceID, value: Buffer): Instruction[] {
        const valueBuf = this.struct.costValue.value.toBytesLE();
        return [
            Instruction.createInvoke(
                coin.id,
                CoinInstance.contractID,
                CoinInstance.commandFetch,
                [new Argument({name: CoinInstance.argumentCoins, value: Buffer.from(valueBuf)})],
            ),
            Instruction.createSpawn(
                this.id,
                ValueInstance.contractID,
                [
                    new Argument({name: ValueInstance.argumentValue, value}),
                ],
            ),
        ];
    }

    /**
     * Create a value instance for a given darc
     *
     * @param coin      The coin instance to take the coins from
     * @param signers   The signers for the transaction
     * @param darcID    The darc responsible for this coin
     * @param value     The value to store in the instance
     * @returns a promise that resolves with the new coin instance
     */
    async spawnValue(coin: CoinInstance, signers: Signer[], darcID: InstanceID, value: Buffer):
        Promise<ValueInstance> {
        const ctx = ClientTransaction.make(
            this.rpc.getProtocolVersion(),
            ...this.spawnValueInstructions(coin, darcID, value),
        );
        await ctx.updateCountersAndSign(this.rpc, [signers, []]);
        await this.rpc.sendTransactionAndWait(ctx);

        return ValueInstance.fromByzcoin(this.rpc, ctx.instructions[1].deriveId(), 2);
    }
}

/**
 * Data of a spawner instance
 */
export class SpawnerStruct extends Message<SpawnerStruct> {

    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("personhood.SpawnerStruct", SpawnerStruct, Coin);
    }

    readonly costDarc: Coin;
    readonly costCoin: Coin;
    readonly costCredential: Coin;
    readonly costParty: Coin;
    readonly costRoPaSci: Coin;
    readonly costCWrite: Coin;
    readonly costCRead: Coin;
    readonly costValue: Coin;
    readonly beneficiary: InstanceID;

    constructor(props?: Properties<SpawnerStruct>) {
        super(props);

        /* Protobuf aliases */

        Object.defineProperty(this, "costdarc", {
            get(): Coin {
                return this.costDarc;
            },
            set(value: Coin) {
                this.costDarc = value;
            },
        });

        Object.defineProperty(this, "costcoin", {
            get(): Coin {
                return this.costCoin;
            },
            set(value: Coin) {
                this.costCoin = value;
            },
        });

        Object.defineProperty(this, "costcredential", {
            get(): Coin {
                return this.costCredential;
            },
            set(value: Coin) {
                this.costCredential = value;
            },
        });

        Object.defineProperty(this, "costparty", {
            get(): Coin {
                return this.costParty;
            },
            set(value: Coin) {
                this.costParty = value;
            },
        });

        Object.defineProperty(this, "costropasci", {
            get(): Coin {
                return this.costRoPaSci;
            },
            set(value: Coin) {
                this.costRoPaSci = value;
            },
        });

        Object.defineProperty(this, "costcread", {
            get(): Coin {
                return this.costCRead;
            },
            set(value: Coin) {
                this.costCRead = value;
            },
        });
        Object.defineProperty(this, "costcwrite", {
            get(): Coin {
                return this.costCWrite;
            },
            set(value: Coin) {
                this.costCWrite = value;
            },
        });
        Object.defineProperty(this, "costvalue", {
            get(): Coin {
                return this.costValue;
            },
            set(value: Coin) {
                this.costValue = value;
            },
        });
    }
}

/**
 * Fields of the costs of a spawner instance
 */
export interface ICreateCost {
    costCRead: Long;
    costCWrite: Long;
    costCoin: Long;
    costCredential: Long;
    costDarc: Long;
    costParty: Long;
    costRoPaSci: Long;
    costValue: Long;

    [k: string]: Long;
}

/**
 * Parameters to create a spawner instance
 */
interface ICreateSpawner {
    bc: ByzCoinRPC;
    darcID: InstanceID;
    signers: Signer[];
    costs: ICreateCost;
    beneficiary: InstanceID;

    [k: string]: any;
}

/**
 * Parameters to create a rock-paper-scissors game
 */
interface ICreateRoPaSci {
    desc: string;
    coin: CoinInstance;
    signers: Signer[];
    stake: Long;
    choice: number;
    fillup: Buffer;
    calypso?: LongTermSecret;

    [k: string]: any;
}

/**
 * Parameters to create a pop party
 */
interface ICreatePopParty {
    coin: CoinInstance;
    signers: Signer[];
    orgs: InstanceID[];
    desc: PopDesc;
    reward: Long;

    [k: string]: any;
}

/**
 * Parameters to create a calypso write instance
 */
interface ISpawnCalyspoWrite {
    coin: CoinInstance;
    signers: Signer[];
    write: Write;
    darcID: InstanceID;
    choice: number;

    [k: string]: any;
}

SpawnerStruct.register();

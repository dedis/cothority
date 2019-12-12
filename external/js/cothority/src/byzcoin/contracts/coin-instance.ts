import { createHash } from "crypto-browserify";
import Long from "long";
import { Message, Properties } from "protobufjs/light";
import Signer from "../../darc/signer";
import { EMPTY_BUFFER, registerMessage } from "../../protobuf";
import ByzCoinRPC from "../byzcoin-rpc";
import ClientTransaction, { Argument, Instruction } from "../client-transaction";
import Instance, { InstanceID } from "../instance";

export default class CoinInstance extends Instance {
    static readonly contractID = "coin";
    static readonly commandMint = "mint";
    static readonly commandFetch = "fetch";
    static readonly commandTransfer = "transfer";
    static readonly commandStore = "store";
    static readonly argumentCoinID = "coinID";
    static readonly argumentDarcID = "darcID";
    static readonly argumentType = "type";
    static readonly argumentCoins = "coins";
    static readonly argumentDestination = "destination";

    /**
     * Generate the coin instance ID for a given darc ID
     *
     * @param buf Any buffer that is known to the caller
     * @returns the id as a buffer
     */
    static coinIID(buf: Buffer): InstanceID {
        const h = createHash("sha256");
        h.update(Buffer.from(CoinInstance.contractID));
        h.update(buf);
        return h.digest();
    }

    /**
     * Spawn a coin instance from a darc id
     *
     * @param bc        The RPC to use
     * @param darcID    The darc instance ID
     * @param signers   The list of signers for the transaction
     * @param type      The coin instance type
     * @returns a promise that resolves with the new instance
     */
    static async spawn(
        bc: ByzCoinRPC,
        darcID: InstanceID,
        signers: Signer[],
        type: Buffer,
    ): Promise<CoinInstance> {
        const inst = Instruction.createSpawn(
            darcID,
            CoinInstance.contractID,
            [new Argument({name: CoinInstance.argumentType, value: type})],
        );
        await inst.updateCounters(bc, signers);

        const ctx = ClientTransaction.make(bc.getProtocolVersion(), inst);
        ctx.signWith([signers]);

        await bc.sendTransactionAndWait(ctx, 10);

        return CoinInstance.fromByzcoin(bc, ctx.instructions[0].deriveId(), 1);
    }

    /**
     * Create returns a CoinInstance from the given parameters.
     * @param bc
     * @param coinID
     * @param darcID
     * @param coin
     */
    static create(
        bc: ByzCoinRPC,
        coinID: InstanceID,
        darcID: InstanceID,
        coin: Coin,
    ): CoinInstance {
        return new CoinInstance(bc, new Instance({
            contractID: CoinInstance.contractID,
            darcID,
            data: coin.toBytes(),
            id: coinID,
        }));
    }

    /**
     * Initializes using an existing coinInstance from ByzCoin
     * @param bc    The RPC to use
     * @param iid   The instance ID
     * @param waitMatch how many times to wait for a match - useful if its called just after an addTransactionAndWait.
     * @param interval how long to wait between two attempts in waitMatch.
     * @returns a promise that resolves with the coin instance
     */
    static async fromByzcoin(bc: ByzCoinRPC, iid: InstanceID, waitMatch: number = 0, interval: number = 1000):
        Promise<CoinInstance> {
        return new CoinInstance(bc, await Instance.fromByzcoin(bc, iid, waitMatch, interval));
    }

    _coin: Coin;

    /**
     * @return value of the coin.
     */
    get value(): Long {
        return this._coin.value;
    }

    /**
     * @return the name of the coin, which is a 32-byte Buffer.
     */
    get name(): Buffer {
        return this._coin.name;
    }

    /**
     * Constructs a new CoinInstance. If the instance is not of type CoinInstance,
     * an error will be thrown.
     *
     * @param rpc a working RPC instance
     * @param inst an instance representing a CoinInstance
     */
    constructor(private rpc: ByzCoinRPC, inst: Instance) {
        super(inst);
        if (inst.contractID.toString() !== CoinInstance.contractID) {
            throw new Error(`mismatch contract name: ${inst.contractID} vs ${CoinInstance.contractID}`);
        }

        this._coin = Coin.decode(inst.data);
    }

    /**
     * Transfer a certain amount of coin to another account.
     *
     * @param coins     the amount
     * @param to        the destination account (must be a coin contract instance id)
     * @param signers   the signers (of the giver account)
     */
    async transfer(coins: Long, to: Buffer, signers: Signer[]): Promise<void> {
        const args = [
            new Argument({name: CoinInstance.argumentCoins, value: Buffer.from(coins.toBytesLE())}),
            new Argument({name: CoinInstance.argumentDestination, value: to}),
        ];

        const inst = Instruction.createInvoke(this.id, CoinInstance.contractID, CoinInstance.commandTransfer, args);
        await inst.updateCounters(this.rpc, signers);

        const ctx = ClientTransaction.make(this.rpc.getProtocolVersion(), inst);
        ctx.signWith([signers]);

        await this.rpc.sendTransactionAndWait(ctx, 10);
    }

    /**
     * Mine a given amount of coins
     *
     * @param signers   The list of signers for the transaction
     * @param amount    The amount to add to the coin instance
     * @param wait      Number of blocks to wait for inclusion
     */
    async mint(signers: Signer[], amount: Long, wait?: number): Promise<void> {
        const inst = Instruction.createInvoke(
            this.id,
            CoinInstance.contractID,
            CoinInstance.commandMint,
            [new Argument({name: CoinInstance.argumentCoins, value: Buffer.from(amount.toBytesLE())})],
        );
        await inst.updateCounters(this.rpc, signers);

        const ctx = ClientTransaction.make(this.rpc.getProtocolVersion(), inst);
        ctx.signWith([signers]);

        await this.rpc.sendTransactionAndWait(ctx, wait);
    }

    /**
     * Update the data of this instance
     *
     * @returns the updated instance
     */
    async update(): Promise<CoinInstance> {
        const p = await this.rpc.getProofFromLatest(this.id);
        if (!p.exists(this.id)) {
            throw new Error("fail to get a matching proof");
        }

        this._coin = Coin.decode(p.value);
        return this;
    }
}

export class Coin extends Message<Coin> {
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("byzcoin.Coin", Coin);
    }

    name: Buffer;
    value: Long;

    constructor(props?: Properties<Coin>) {
        super(props);

        this.name = Buffer.from(this.name || EMPTY_BUFFER);
    }

    toBytes(): Buffer {
        return Buffer.from(Coin.encode(this).finish());
    }
}

Coin.register();

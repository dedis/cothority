import { Message, Properties } from "protobufjs/light";
import ByzCoinRPC from "../byzcoin/byzcoin-rpc";
import ClientTransaction, { Argument, Instruction } from "../byzcoin/client-transaction";
import CoinInstance, { Coin } from "../byzcoin/contracts/coin-instance";
import Instance, { InstanceID } from "../byzcoin/instance";
import Signer from "../darc/signer";
import { EMPTY_BUFFER, registerMessage } from "../protobuf";

export default class RoPaSciInstance extends Instance {

    get stake(): Coin {
        return this.struct.stake;
    }

    get playerChoice(): number {
        return this.struct.firstPlayer;
    }

    /**
     * Getter for the second player ID
     * @returns id as a buffer
     */
    get adversaryID(): Buffer {
        return this.struct.secondPlayerAccount;
    }

    /**
     * Getter for the second player choice
     * @returns the choice as a number
     */
    get adversaryChoice(): number {
        return this.struct.secondPlayer;
    }
    static readonly contractID = "ropasci";

    /**
     * Fetch the proof for the given instance and create a
     * RoPaSciInstance from it
     *
     * @param bc    The ByzCoinRPC to use
     * @param iid   The instance ID
     * @param waitMatch how many times to wait for a match - useful if its called just after an addTransactionAndWait.
     * @param interval how long to wait between two attempts in waitMatch.
     * @returns the new instance
     */
    static async fromByzcoin(bc: ByzCoinRPC, iid: InstanceID, waitMatch: number = 0, interval: number = 1000):
        Promise<RoPaSciInstance> {
        return new RoPaSciInstance(bc, await Instance.fromByzcoin(bc, iid, waitMatch, interval));
    }

    struct: RoPaSciStruct;
    private fillUp: Buffer;
    private firstMove: number;

    constructor(private rpc: ByzCoinRPC, inst: Instance) {
        super(inst);
        if (inst.contractID.toString() !== RoPaSciInstance.contractID) {
            throw new Error(`mismatch contract name: ${inst.contractID} vs ${RoPaSciInstance.contractID}`);
        }

        this.struct = RoPaSciStruct.decode(this.data);
    }

    /**
     * Update the instance data
     *
     * @param choice The choice of the first player
     * @param fillup The fillup of the first player
     */
    setChoice(choice: number, fillup: Buffer) {
        this.firstMove = choice;
        this.fillUp = fillup;
    }

    /**
     * Returns the firstMove and the fillUp values.
     */
    getChoice(): [number, Buffer] {
        return [this.firstMove, this.fillUp ? Buffer.from(this.fillUp) : undefined];
    }

    /**
     * Check if both players have played their moves
     *
     * @returns true when both have played, false otherwise
     */
    isDone(): boolean {
        return this.struct.secondPlayer >= 0;
    }

    /**
     * Play the adversary move
     *
     * @param coin      The CoinInstance of the second player
     * @param signer    Signer for the transaction
     * @param choice    The choice of the second player
     * @returns a promise that resolves on success, or rejects with the error
     */
    async second(coin: CoinInstance, signer: Signer, choice: number): Promise<void> {
        if (!coin.name.equals(this.struct.stake.name)) {
            throw new Error("not correct coin-type for player 2");
        }
        if (coin.value.lessThan(this.struct.stake.value)) {
            throw new Error("don't have enough coins to match stake");
        }

        const ctx = ClientTransaction.make(
            this.rpc.getProtocolVersion(),
            Instruction.createInvoke(
                coin.id,
                CoinInstance.contractID,
                CoinInstance.commandFetch,
                [
                    new Argument({
                        name: CoinInstance.argumentCoins,
                        value: Buffer.from(this.struct.stake.value.toBytesLE()),
                    }),
                ],
            ),
            Instruction.createInvoke(
                this.id,
                RoPaSciInstance.contractID,
                "second",
                [
                    new Argument({name: "account", value: coin.id}),
                    new Argument({name: "choice", value: Buffer.from([choice % 3])}),
                ],
            ),
        );
        await ctx.updateCountersAndSign(this.rpc, [[signer], []]);

        await this.rpc.sendTransactionAndWait(ctx);
    }

    /**
     * Reveal the move of the first player
     *
     * @param coin The CoinInstance of the first player
     * @returns a promise that resolves on success, or rejects
     * with the error
     */
    async confirm(coin: CoinInstance): Promise<void> {
        if (!coin.name.equals(this.struct.stake.name)) {
            throw new Error("not correct coin-type for player 1");
        }

        const preHash = Buffer.alloc(32, 0);
        preHash[0] = this.firstMove % 3;
        this.fillUp.copy(preHash, 1);
        const ctx = ClientTransaction.make(this.rpc.getProtocolVersion(), Instruction.createInvoke(
            this.id,
            RoPaSciInstance.contractID,
            "confirm",
            [
                new Argument({name: "prehash", value: preHash}),
                new Argument({name: "account", value: coin.id}),
            ],
        ));
        await this.rpc.sendTransactionAndWait(ctx);
        await this.update();
    }

    /**
     * Update the state of the instance
     *
     * @returns a promise that resolves with the updated instance,
     * or rejects with the error
     */
    async update(): Promise<RoPaSciInstance> {
        const proof = await this.rpc.getProofFromLatest(this.id);
        if (!proof.exists(this.id)) {
            throw new Error("fail to get a matching proof");
        }

        const inst = Instance.fromProof(this.id, proof);
        this.data = inst.data;
        this.struct = RoPaSciStruct.decode(this.data);
        return this;
    }
}

/**
 * Data hold by a rock-paper-scissors instance
 */
export class RoPaSciStruct extends Message<RoPaSciStruct> {
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("personhood.RoPaSciStruct", RoPaSciStruct);
    }

    readonly description: string;
    readonly stake: Coin;
    readonly firstPlayerHash: Buffer;
    readonly firstPlayer: number;
    readonly secondPlayer: number;
    readonly secondPlayerAccount: Buffer;

    constructor(props?: Properties<RoPaSciStruct>) {
        super(props);

        this.firstPlayerHash = Buffer.from(this.firstPlayerHash || EMPTY_BUFFER);
        this.secondPlayerAccount = Buffer.from(this.secondPlayerAccount || EMPTY_BUFFER);

        Object.defineProperty(this, "firstplayer", {
            get(): number {
                return this.firstPlayer;
            },
            set(value: number) {
                this.firstPlayer = value;
            },
        });

        Object.defineProperty(this, "firstplayerhash", {
            get(): Buffer {
                return this.firstPlayerHash;
            },
            set(value: Buffer) {
                this.firstPlayerHash = value;
            },
        });

        Object.defineProperty(this, "secondplayer", {
            get(): number {
                return this.secondPlayer;
            },
            set(value: number) {
                this.secondPlayer = value;
            },
        });

        Object.defineProperty(this, "secondplayeraccount", {
            get(): Buffer {
                return this.secondPlayerAccount;
            },
            set(value: Buffer) {
                this.secondPlayerAccount = value;
            },
        });
    }

    /**
     * Helper to encode the struct using protobuf
     *
     * @returns the data as a buffer
     */
    toBytes(): Buffer {
        return Buffer.from(RoPaSciStruct.encode(this).finish());
    }
}

RoPaSciStruct.register();

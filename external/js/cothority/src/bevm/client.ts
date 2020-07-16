import { randomBytes } from "crypto";
import { ec } from "elliptic";
import Keccak from "keccak";
import Long from "long";

import ByzCoinRPC from "../byzcoin/byzcoin-rpc";
import ClientTransaction, { Argument, Instruction } from "../byzcoin/client-transaction";
import Instance, { InstanceID } from "../byzcoin/instance";
import Signer from "../darc/signer";
import Log from "../log";

import { BEvmService } from "./service";

/**
 * Ethereum account
 */
export class EvmAccount {
    static EC = new ec("secp256k1");

    static deserialize(obj: any): EvmAccount {
        return new EvmAccount(obj.name, obj.privateKey, obj.nonce);
    }

    private static computeAddress(key: ec.KeyPair): Buffer {
        // Marshal public key to binary
        const pubBytes = Buffer.from(key.getPublic("hex"), "hex");

        const h = new Keccak("keccak256");
        h.update(pubBytes.slice(1));

        const address = h.digest().slice(12);
        Log.lvl3("Computed account address", address.toString("hex"));

        return address;
    }

    readonly address: Buffer;
    protected key: ec.KeyPair;
    private _nonce: number;

    get nonce() {
        return this._nonce;
    }

    /**
     * Create a new EVM account
     *
     * @param name      Account name
     * @param privKey   Private key for the account (32 bytes); if not
     *                  specified, a new one is generated randomly
     * @param nonce     Initial nonce for the account; should normally not be specified
     */
    constructor(readonly name: string, privKey?: Buffer, nonce: number = 0) {
        if (privKey === undefined) {
            privKey = randomBytes(32);
        }
        this.key = EvmAccount.EC.keyFromPrivate(privKey);

        this.address = EvmAccount.computeAddress(this.key);
        this._nonce = nonce;
    }

    sign(hash: Buffer): Buffer {
        /* WARNING: The "canonical" option is crucial to have the same
        * signature as Ethereum */
        const sig = this.key.sign(hash, {canonical: true});

        const r = Buffer.from(sig.r.toArray("be", 32));
        const s = Buffer.from(sig.s.toArray("be", 32));

        const len = r.length + s.length + 1;

        const buf = Buffer.concat([r, s], len);
        buf.writeUInt8(sig.recoveryParam, len - 1);

        return buf;
    }

    incNonce() {
        this._nonce += 1;
    }

    serialize(): object {
        return {
            name: this.name,
            nonce: this.nonce,
            privateKey: this.key.getPrivate("hex"),
        };
    }
}

/**
 * Ethereum smart contract
 */
export class EvmContract {
    static deserialize(obj: any): EvmContract {
        const addresses = obj.addresses.map((elem: string) => Buffer.from(elem, "hex"));
        const bytecode = Buffer.from(obj.bytecode, "hex");

        const contract = new EvmContract(obj.name, bytecode, obj.abi, addresses);

        return contract;
    }

    private static computeAddress(data: Buffer, nonce: number): Buffer {
        const buf = EvmContract.erlEncode(data, nonce);

        const h = new Keccak("keccak256");
        h.update(buf);

        const address = h.digest().slice(12);
        Log.lvl3("Computed contract address", address.toString("hex"));

        return address;
    }

    // Translated from the Go Ethereum code
    private static erlEncode(address: Buffer, nonce: number): Buffer {
        const bufNonce = Buffer.alloc(8);
        bufNonce.writeUInt32BE(nonce / (2 ** 32), 0);
        bufNonce.writeUInt32BE(nonce % (2 ** 32), 4);
        let size = 8;
        for (let i = 0; (i < 8) && (bufNonce[i] === 0); i++) {
            size--;
        }

        const addressLen = address.length + 1;
        const nonceLen = (nonce < 128 ? 1 : size + 1);

        const buf = Buffer.alloc(1 + addressLen + nonceLen);
        let pos = 0;

        buf.writeUInt8(0xc0 + addressLen + nonceLen, pos++);

        buf.writeUInt8(0x80 + address.length, pos++);
        address.copy(buf, 2);
        pos += address.length;

        if ((nonce === 0) || (nonce >= 128)) {
            buf.writeUInt8(0x80 + size, pos++);
        }

        bufNonce.copy(buf, pos, 8 - size);

        return buf;
    }

    readonly transactions: string[];
    readonly viewMethods: string[];

    /**
     * Create a new EVM contract
     *
     * @param name      Contract name
     * @param bytecode  Contract bytecode
     * @param abi       Contract ABI (JSON-encoded)
     * @param addresses Array of deployed contract instances; should normally not be specified
     */
    constructor(readonly name: string,
                readonly bytecode: Buffer,
                readonly abi: string,
                readonly addresses: Buffer[] = []) {
        const abiObj = JSON.parse(abi);

        const transactions = abiObj.filter((elem: any) => {
            return elem.type === "function" &&  elem.stateMutability !== "view";
        }).map((elem: any) => elem.name);
        this.transactions = transactions;

        const viewMethods = abiObj.filter((elem: any) => {
            return elem.type === "function" &&  elem.stateMutability === "view";
        }).map((elem: any) => elem.name);
        this.viewMethods = viewMethods;
    }

    createNewAddress(account: EvmAccount) {
        const newAddress = EvmContract.computeAddress(account.address, account.nonce);
        this.addresses.push(newAddress);
    }

    serialize(): object {
        return {
            abi: this.abi,
            addresses: this.addresses.map((address) => address.toString("hex")),
            bytecode: this.bytecode.toString("hex"),
            name: this.name,
        };
    }
}

// Number of WEIs in one ETHER
export const WEI_PER_ETHER = Long.fromString("1000000000000000000");

/**
 * BEvm client
 */
export class BEvmClient extends Instance {
    static readonly contractID = "bevm";

    static readonly commandTransaction = "transaction";
    static readonly argumentTx = "tx";

    static readonly commandCredit = "credit";
    static readonly argumentAddress = "address";
    static readonly argumentAmount = "amount";

    /**
     * Spawn a new BEvm instance
     *
     * @param bc        ByzCoin RPC to use
     * @param darcID    DARC instance ID
     * @param signers   List of signers for the ByzCoin transaction
     *
     * @return New BEvm instance
     */
    static async spawn(bc: ByzCoinRPC, darcID: InstanceID, signers: Signer[]): Promise<BEvmClient> {
        const inst = Instruction.createSpawn(
            darcID,
            BEvmClient.contractID,
            [],
        );

        const ctx = ClientTransaction.make(bc.getProtocolVersion(), inst);
        await ctx.updateCountersAndSign(bc, [signers]);

        await bc.sendTransactionAndWait(ctx);

        return BEvmClient.fromByzcoin(bc, ctx.instructions[0].deriveId(), 2);
    }

    /**
     * Retrieve an existing BEvm instance
     *
     * @param bc    ByzCoin RPC to use
     * @param iid   BEvm instance ID
     *
     * @returns BEvm instance
     */
    static async fromByzcoin(bc: ByzCoinRPC, iid: InstanceID,
                             waitMatch: number = 0,
                             interval: number = 1000): Promise<BEvmClient> {
        const instance = await Instance.fromByzcoin(bc, iid, waitMatch, interval);

        return new BEvmClient(bc, instance);
    }

    private bevmService: BEvmService;

    constructor(private byzcoinRPC: ByzCoinRPC, inst: Instance) {
        super(inst);

        if (inst.contractID.toString() !== BEvmClient.contractID) {
            throw new Error(`mismatch contract name: ${inst.contractID} vs ${BEvmClient.contractID}`);
        }
    }

    /**
     * Set the BEvm service to use
     */
    setBEvmService(bevmService: BEvmService) {
        this.bevmService = bevmService;
    }

    /**
     * Deploy an EVM smart contract
     *
     * @param signers   ByzCoin identities signing the ByzCoin transaction
     * @param gasLimit  Gas limit for the transaction
     * @param gasPrice  Gas price for the transaction
     * @param amount    Amount transferred during the transaction
     * @param account   EVM account
     * @param contract  EVM contract to deploy
     * @param args      Arguments for the smart contract constructor
     * @param wait      Number of blocks to wait for the ByzCoin transaction to be included
     *
     * The `args` are passed as an array of strings, one per argument, each of
     * them JSON-encoded.
     * The following argument types are currently supported:
     *
     *   Solidity type          | JSON type | Example
     *   --------------------------------------------
     *   uint, uint256, uint128 | string    | "12345"
     *   int, int256, int128    | string    | "-12345"
     *   uint32, uint16, uint8  | number    | 12345
     *   int32, int16, int8     | number    | -12345
     *   address                | string    | "112233445566778899aabbccddeeff0011223344"
     *   string                 | string    | "look at me I am a string"
     *   array, e.g. uint[2]    | array     | ["123", "456"]
     */
    async deploy(signers: Signer[],
                 gasLimit: number,
                 gasPrice: number,
                 amount: number,
                 account: EvmAccount,
                 contract: EvmContract,
                 args?: string[],
                 wait?: number) {
        const unsignedTx = await this.bevmService.prepareDeployTx(
            gasLimit, gasPrice, amount, account.nonce,
            contract.bytecode, contract.abi, args);
        const signature = account.sign(Buffer.from(unsignedTx.transactionHash));
        const signedTx = await this.bevmService.finalizeTx(
            Buffer.from(unsignedTx.transaction), signature);

        await this.invoke(
            BEvmClient.commandTransaction, [
                new Argument({name: BEvmClient.argumentTx,
                             value: Buffer.from(signedTx.transaction)}),
            ],
            signers, wait);

        contract.createNewAddress(account);
        account.incNonce();
    }

    /**
     * Execute an EVM transaction on a deployed smart contract (R/W method)
     *
     * @param signers       ByzCoin identities signing the ByzCoin transaction
     * @param gasLimit      Gas limit for the transaction
     * @param gasPrice      Gas price for the transaction
     * @param amount        Amount transferred during the transaction
     * @param account       EVM account
     * @param contract      EVM contract
     * @param instanceIndex Index of the deployed smart contract instance
     * @param method        Name of the method to execute
     * @param args          Arguments for the smart contract method
     * @param wait          Number of blocks to wait for the ByzCoin transaction to be included
     *
     * See `deploy()` for a description of `args`.
     */
    async transaction(signers: Signer[],
                      gasLimit: number,
                      gasPrice: number,
                      amount: number,
                      account: EvmAccount,
                      contract: EvmContract,
                      instanceIndex: number,
                      method: string,
                      args?: string[],
                      wait?: number) {
        const contractAddress = contract.addresses[instanceIndex];
        const unsignedTx = await this.bevmService.prepareTransactionTx(
            gasLimit, gasPrice, amount, contractAddress, account.nonce,
            contract.abi, method, args);
        const signature = account.sign(Buffer.from(unsignedTx.transactionHash));
        const signedTx = await this.bevmService.finalizeTx(
            Buffer.from(unsignedTx.transaction), signature);

        await this.invoke(
            BEvmClient.commandTransaction, [
                new Argument({name: BEvmClient.argumentTx,
                             value: Buffer.from(signedTx.transaction)}),
            ],
            signers, wait);

        account.incNonce();
    }

    /**
     * Execute a view method on a deployed smart contract (read-only method)
     *
     * A view method is executed on a conode on behalf of the client. This
     * conode does not necessarily belong to the ByzCoin cothority. Therefore,
     * the actual cothority configuration, the BEvm ID as well as the ByzCoin
     * ID that contains it must be provided.
     *
     * @param byzcoinId         ByzCoin ID
     * @param serverConfig      Cothority server config in TOML
     * @param bevmInstanceId    BEvm instance ID
     * @param account           EVM account
     * @param contract          EVM contract
     * @param instanceIndex     Index of the deployed smart contract instance
     * @param method            Name of the view method to execute
     * @param args              Arguments for the smart contract method
     *
     * @return Result of the view method execution
     *
     * See `deploy()` for a description of `args`.
     */
    async call(byzcoinId: Buffer,
               serverConfig: string,
               bevmInstanceId: Buffer,
               account: EvmAccount,
               contract: EvmContract,
               instanceIndex: number,
               method: string,
               args?: string[]): Promise<any> {
        const contractAddress = contract.addresses[instanceIndex];

        const response = await this.bevmService.performCall(
            byzcoinId,
            serverConfig,
            bevmInstanceId,
            account.address,
            contractAddress,
            contract.abi,
            method,
            args);

        return JSON.parse(response.result);
    }

    /**
     * Credit an EVM account by the specified amount
     *
     * @param signers   ByzCoin identities signing the ByzCoin transaction
     * @param account   EVM account
     * @param amount    Amount to credit on the account
     * @param wait      Number of blocks to wait for the ByzCoin transaction to be included
     */
    async creditAccount(signers: Signer[],
                        account: EvmAccount,
                        amount: Long,
                        wait?: number) {
        const amountBuf = Buffer.from(amount.toBytesBE());

        await this.invoke(
            BEvmClient.commandCredit,
            [
                new Argument({name: BEvmClient.argumentAddress,
                             value: account.address}),
                new Argument({name: BEvmClient.argumentAmount,
                             value: amountBuf}),
            ],
            signers,
            wait);
    }

    private async invoke(command: string,
                         args: Argument[],
                         signers: Signer[],
                         wait?: number) {
        const ctx = ClientTransaction.make(
            this.byzcoinRPC.getProtocolVersion(),
            Instruction.createInvoke(
                this.id, BEvmClient.contractID, command, args,
            ));

        await ctx.updateCountersAndSign(this.byzcoinRPC, [signers]);
        await this.byzcoinRPC.sendTransactionAndWait(ctx, wait);
    }
}

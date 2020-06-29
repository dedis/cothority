import Log from "../log";
import { ServerIdentity, WebSocketConnection } from "../network";
import {
    CallRequest,
    CallResponse,
    DeployRequest,
    TransactionFinalizationRequest,
    TransactionHashResponse,
    TransactionRequest,
    TransactionResponse,
} from "./proto";

/**
 * BEvm service
 */
export class BEvmService {
    static serviceName = "BEvm";

    private conn: WebSocketConnection;
    private timeout: number;

    constructor(srvid: ServerIdentity) {
        this.timeout = 60 * 1000; // 60 seconds
        this.conn = new WebSocketConnection(srvid.getWebSocketAddress(),
                                            BEvmService.serviceName);
    }

    /**
     * Set a new timeout value for future requests.
     *
     * @param value Timeout in [ms]
     */
    setTimeout(value: number): void {
        this.timeout = value;
    }

    /**
     * Prepare an EVM transaction to deploy a contract.
     *
     * @param gasLimit  Gas limit for the transaction
     * @param gasPrice  Gas price for the transaction
     * @param amount    Amount transferred during the transaction
     * @param nonce     Nonce for the transaction
     * @param bytecode  Compiled bytecode of the smart contract to be deployed
     * @param abi       ABI of the smart contract to be deployed
     * @param args      Arguments for the smart contract contractor
     *
     * @return Prepared transaction and its hash to be signed
     */
    async prepareDeployTx(gasLimit: number,
                          gasPrice: number,
                          amount: number,
                          nonce: number,
                          bytecode: Buffer,
                          abi: string,
                          args: string[]): Promise<TransactionHashResponse> {
        this.conn.setTimeout(this.timeout);

        Log.lvl3("Sending BEvm deploy contract request...");

        const msg = new DeployRequest({
            abi,
            amount,
            args,
            bytecode,
            gasLimit,
            gasPrice,
            nonce,
        });

        return this.conn.send(msg, TransactionHashResponse);
    }

    /**
     * Prepare an EVM transaction for a R/W method execution
     *
     * @param gasLimit          Gas limit for the transaction
     * @param gasPrice          Gas price for the transaction
     * @param amount            Amount transferred during the transaction
     * @param contractAddress   Address of the smart contract
     * @param nonce             Nonce for the transaction
     * @param abi               ABI of the smart contract to be deployed
     * @param method            Name of the method to execute
     * @param args              Arguments for the smart contract method
     *
     * @return Prepared transaction and its hash to be signed
     */
    async prepareTransactionTx(gasLimit: number,
                               gasPrice: number,
                               amount: number,
                               contractAddress: Buffer,
                               nonce: number,
                               abi: string,
                               method: string,
                               args: string[]): Promise<TransactionHashResponse> {
        this.conn.setTimeout(this.timeout);

        Log.lvl3("Sending BEvm transaction execution request...");

        const msg = new TransactionRequest({
                abi,
                amount,
                args,
                contractAddress,
                gasLimit,
                gasPrice,
                method,
                nonce,
            });

        return this.conn.send(msg, TransactionHashResponse);
    }

    /**
     * Finalize a transaction
     *
     * @param transaction   Transaction to finalize
     * @param signature     Transaction signature
     *
     * @return Signed transaction
     */
    async finalizeTx(transaction: Buffer,
                     signature: Buffer): Promise<TransactionResponse> {
        this.conn.setTimeout(this.timeout);

        Log.lvl3("Sending BEvm transaction finalization request...");

        const msg = new TransactionFinalizationRequest({
                signature,
                transaction,
            });

        return this.conn.send(msg, TransactionResponse);
    }

    /**
     * Execute a view method (read-only)
     *
     * @param byzcoinId         ByzCoin ID
     * @param serverConfig      Cothority server config in TOML
     * @param bevmInstanceId    BEvm instance ID
     * @param accountAddress    Address of the EVM account
     * @param contractAddress   Address of the smart contract
     * @param abi               ABI of the smart contract to be deployed
     * @param method            Name of the view method to execute
     * @param args              Arguments for the smart contract method
     *
     * @return Result of the view method execution
     */
    async performCall(byzcoinId: Buffer,
                      serverConfig: string,
                      bevmInstanceId: Buffer,
                      accountAddress: Buffer,
                      contractAddress: Buffer,
                      abi: string,
                      method: string,
                      args: string[]):
                          Promise<CallResponse> {
        this.conn.setTimeout(this.timeout);

        Log.lvl3("Sending BEvm call request...");

        const msg = new CallRequest({
                abi,
                accountAddress,
                args,
                bevmInstanceId,
                byzcoinId,
                contractAddress,
                method,
                serverConfig,
            });

        return this.conn.send(msg, CallResponse);
    }
}

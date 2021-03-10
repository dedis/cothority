import Log from "../log";
import { Roster, WebSocketConnection } from "../network";
import { ViewCallRequest, ViewCallResponse } from "./proto";

/**
 * BEvm service
 */
export class BEvmRPC {
    static serviceName = "BEvm";

    // Simple implementation of Promise.any()
    // Return a promise that resolves when the first given promise resolves,
    // or fails when they all do.
    static anyPromise<T>(promises: Array<Promise<T>>): Promise<T> {
        // Number of promises, used to determine when all have failed
        let count = promises.length;
        // Array to collect the promise failures
        const errors: any[] = Array.from({length: count});

        if (count === 0) {
            return Promise.reject("Empty list of promises");
        }

        return new Promise(
            (resolve, reject) => promises.forEach(
                (p, index) => p.then(
                    (value) => resolve(value),
                    (reason) => {
                        errors[index] = reason;
                        count--;
                        if (count === 0) {
                            reject(errors);
                        }
                    })));
    }

    readonly conns: WebSocketConnection[];
    private timeout: number;

    constructor(roster: Roster) {
        this.timeout = 60 * 1000; // 60 seconds
        this.conns = roster.list.map(
            (srvid) => new WebSocketConnection(srvid.getWebSocketAddress(),
                                               BEvmRPC.serviceName));
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
     * Execute a view method (read-only)
     *
     * @param byzcoinId         ByzCoin ID
     * @param bevmInstanceId    BEvm instance ID
     * @param accountAddress    Address of the EVM account
     * @param contractAddress   Address of the smart contract
     * @param callData          ABI-packed call arguments
     * @param minBlockIndex     Minimum block index that nodes must have to
     *                          execute the view method
     *
     * @return Result of the view method execution
     */
    async viewCall(byzcoinId: Buffer,
                   bevmInstanceId: Buffer,
                   accountAddress: Buffer,
                   contractAddress: Buffer,
                   callData: Buffer,
                   minBlockIndex = 0):
                       Promise<ViewCallResponse> {
        this.conns.forEach( (conn) => conn.setTimeout(this.timeout) );

        Log.lvl3("Sending BEvm call request...");

        const msg = new ViewCallRequest({
                accountAddress,
                bevmInstanceId,
                byzcoinId,
                callData,
                contractAddress,
                minBlockIndex,
            });

        // Send to the whole roster, use the first answer received
        return BEvmRPC.anyPromise(this.conns.map(
            (conn) => conn.send<ViewCallResponse>(msg, ViewCallResponse)));
    }
}

import { Log } from "../../src";
import { ByzCoinRPC, IStorage, LocalCache } from "../../src/byzcoin";
import { Darc, Rule } from "../../src/darc";
import { RosterWSConnection } from "../../src/network";
import { StatusRPC } from "../../src/status";
import { StatusRequest, StatusResponse } from "../../src/status/proto";
import { TransactionBuilder } from "../../src/v2/byzcoin";
import { CoinContract, DarcInst } from "../../src/v2/byzcoin/contracts";
import { BLOCK_INTERVAL, ROSTER, SIGNER, startConodes, stopConodes } from "../support/conondes";

/**
 * BCTest allows for using a single ByzCoin instance for multiple tests. It should be called with
 *
 *   const bct = await BCTest.singleton()
 *
 * in every test where a byzcoin-instance is used. Thereafter the test can use the genesisInst
 * to create new CoinInstances and DarcInstances.
 *
 * Using this class reduces the time to test, as the same ByzCoin instance is used for all tests.
 * But it also means that the tests need to make sure that the genesis-darc is not made
 * unusable.
 */
export class BCTest {

    static async singleton(): Promise<BCTest> {
        if (BCTest.bct === undefined) {
            BCTest.bct = await BCTest.init();
        } else {
            await new Promise((resolve) => setTimeout(resolve, 1000));
        }

        return BCTest.bct;
    }
    private static bct: BCTest | undefined;

    private static async init(): Promise<BCTest> {
        Log.lvl = 1;
        const roster4 = ROSTER.slice(0, 4);

        let usesDocker = true;
        try {
            const ws = new RosterWSConnection(roster4, StatusRPC.serviceName);
            ws.setParallel(1);
            await ws.send(new StatusRequest(), StatusResponse);
            Log.warn("Using already running nodes for test!");
            usesDocker = false;
        } catch (e) {
            await startConodes();
        }

        const cache = new LocalCache();
        const genesis = ByzCoinRPC.makeGenesisDarc([SIGNER], roster4, "initial");
        [CoinContract.ruleFetch, CoinContract.ruleMint, CoinContract.ruleSpawn, CoinContract.ruleStore,
            CoinContract.ruleTransfer]
            .forEach((rule) => genesis.addIdentity(rule, SIGNER, Rule.OR));
        const rpc = await ByzCoinRPC.newByzCoinRPC(roster4, genesis, BLOCK_INTERVAL, cache);
        rpc.setParallel(1);
        const tx = new TransactionBuilder(rpc);
        const genesisInst = await DarcInst.retrieve(rpc, genesis.getBaseID());
        return new BCTest(cache, genesis, genesisInst, rpc, tx, usesDocker);
    }

    private constructor(
        public cache: IStorage,
        public genesis: Darc,
        public genesisInst: DarcInst,
        public rpc: ByzCoinRPC,
        public tx: TransactionBuilder,
        public usesDocker: boolean,
    ) {
    }

    async shutdown() {
        if (this.usesDocker) {
            return stopConodes();
        }
    }
}

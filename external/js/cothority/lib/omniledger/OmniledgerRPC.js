const identity = require("../identity");
const net = require("../net");
const Proof = require("./Proof");
const Config = require("./Config");
const Darc = require("./darc/Darc");
const SkipchainClient = require("../skipchain").Client;
const Kyber = require("@dedis/kyber-js");
const misc = require("../misc");
const protobuf = require("protobufjs");

/**
 * OmniledgerRPC interacts with the omniledger service of a conode.
 * It can link to an existing omniledger instance.
 */
class OmniledgerRPC {
  /**
   * Constructs an OmniLedgerRPC when the complete configuration is known
   *
   * @param {Config} config - the configuration of the OmniLedger
   * @param {Socket|LeaderSocket|RosterSocket} socket - the socket to communicate with the OmniLedger
   * @param {Darc} genesisDarc - the genesis Darc
   * @param {Object} genesis - the first block of the skipchain, in Protobuf literral JS object
   * @param {Object} latest - the last block of the skipchain, in Protobuf literral JS object
   * @param {Uint8Array} skipchainID - the ID of the skipchain (aka the
   * ID of the genesis skipblock)
   * @param {SkipchainClient} skipchain - an RPC to talk with the skipchain
   */
  constructor(
    config,
    socket,
    genesisDarc,
    genesis,
    latest,
    skipchainID,
    skipchain
  ) {
    this._config = config;
    this._socket = socket;
    this._genesisDarc = genesisDarc;
    this._genesis = genesis;
    this._latest = latest;
    this._skipchainID = skipchainID;
    this._skipchain = skipchain;
  }

  /**
   * Return the current config of the OmniLedger
   * @return {Config}
   */
  get config() {
    return this._config;
  }

  /**
   * @return {Uint8Array} the ID of the skipchain (aka the ID of the genesis
   * skipblock)
   */
  get skipchainID() {
    return this._skipchainID;
  }

  /**
   *
   * @return {Roster} roster - the roster that hosts the omniledger
   */
  get roster() {
    return this.config.roster;
  }

  /**
   * @return {number}
   */
  static get currentVersion() {
    return 1;
  }

  /**
   * Sends a transaction to omniledger and waits for up to 'wait' blocks for the
   * transaction to be included in the global state. If more than 'wait' blocks
   * are created and the transaction is not included, an exception will be raised.
   *
   * @param {ClientTransaction} transaction - is the client transaction holding
   * one or more instructions to be sent to omniledger.
   * @param {number} wait - indicates the number of blocks to wait for the
   * transaction to be included
   * @return {Promise} - a promise that gets resolved if the block has been included
   */
  sendTransactionAndWait(transaction, wait) {
    let addTxRequest = {
      version: OmniledgerRPC.currentVersion,
      skipchainid: this.skipchainID,
      transaction: transaction.toProtobufValidMessage(),
      inclusionwait: wait
    };
    return this._socket
      .send("AddTxRequest", "AddTxResponse", addTxRequest)
      .then(() => {
        console.log("Successfully stored request - waiting for inclusion");
      })
      .catch(e => {
        if (e instanceof protobuf.util.ProtocolError) {
          console.log(
            "The transaction has not been included within " + wait + " blocks"
          );
        }

        return Promise.reject(e);
      });
  }

  /**
   * Gets a proof from omniledger to show that a given instance is in the
   * global state.

   * @param {Uint8Array} id - the instance key
   * @return {Promise<Proof>}
   */
  getProof(id) {
    return OmniledgerRPC.getProof(this._socket, this._skipchainID, id);
  }

  /**
   * Gets a proof from omniledger to show that a given instance is in the
   * global state.
   *
   * @param {Socket|LeaderSocket|RosterSocket} socket - the socket to communicate with the OmniLedger
   * @param {Uint8Array} skipchainId - the skipchain ID (the ID of it's genesis block)
   * @param {Uint8Array} key - the instance key
   * @return {Promise<Proof>}
   */
  static getProof(socket, skipchainId, key) {
    const getProofMessage = {
      version: OmniledgerRPC.currentVersion,
      id: skipchainId,
      key: key
    };
    return socket
      .send("GetProof", "GetProofResponse", getProofMessage)
      .then(reply => {
        return Promise.resolve(new Proof(reply.proof));
      })
      .catch(err => {
        console.dir("err : " + err);
        console.trace();
        return Promise.reject(err);
      });
  }

  /**
   * Check the validity of the proof
   *
   * @param {Proof} proof
   * @param {string} expectedContract
   * @throws {Error} if the proof is not valid
   */
  static checkProof(proof, expectedContract) {
    if (!proof.matches()) {
      throw "could'nt find darc";
    }
    if (proof.values.length !== 3) {
      throw "incorrect number of values in proof";
    }
    let contract = Array.from(proof.values[1])
      .map(c => String.fromCharCode(c))
      .join("");
    if (!(contract === expectedContract)) {
      throw "contract name is not " + expectedContract + ", got " + contract;
    }
  }

  /**
   * Constructs an OmniLedgerRPC from known configuration. The constructor will communicate with the service to
   * populate other fields and perform verification.
   *
   * @param {Socket|LeaderSocket|RosterSocket} socket - the socket to communicate with the OmniLedger
   * @param skipchainId - the ID of the skipchain (aka the
   * ID of the genesis skipblock)
   * @return {Promise<OmniledgerRPC>} - a promise that gets resolved once the RPC
   * has been created
   */
  static fromKnownConfiguration(socket, skipchainId) {
    if (socket.send === undefined) {
      throw new TypeError("roster must be of type Roster");
    }
    if (!(skipchainId instanceof Uint8Array)) {
      throw new TypeError("skipchainId must be of type UInt8Array");
    }
    let config = undefined;
    let genesisDarc = undefined;
    return this.getProof(socket, skipchainId, new Uint8Array(32))
      .then(proof => {
        OmniledgerRPC.checkProof(proof, "config");
        config = Config.fromByteBuffer(proof.values[0]);

        return OmniledgerRPC.getProof(socket, skipchainId, proof.values[2]);
      })
      .then(proof2 => {
        OmniledgerRPC.checkProof(proof2, "darc");
        genesisDarc = Darc.fromByteBuffer(proof2.values[0]);
        let skipchain = new SkipchainClient(
          Kyber.curve.newCurve("edwards25519"),
          config.roster,
          misc.uint8ArrayToHex(skipchainId)
        );
        let genesis = undefined;
        return skipchain
          .getSkipblock(skipchainId)
          .then(result => {
            genesis = result;

            return skipchain.getLatestBlock();
          })
          .then(latest => {
            return Promise.resolve(
              new OmniledgerRPC(
                config,
                socket,
                genesisDarc,
                genesis,
                latest,
                skipchainId,
                skipchain
              )
            );
          });
      });
  }
}

module.exports = OmniledgerRPC;

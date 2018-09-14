const root = require("../../protobuf").root;
const Instance = require("../Instance");
const Invoke = require("../Invoke");
const Instruction = require("../Instruction");
const ClientTransaction = require("../ClientTransaction");
const crypto = require("crypto");

/**
 * Represents a PoP Party stored on the ledger.
 */
class PopPartyInstance {
  /**
   * @param {ByzCoinRPC} bc - the byzcoin instance
   * @param {Uint8Array} instanceId - the contract instance id
   * @param {Instance} [instance] - the complete instance
   * @param {number} [state] - the state of the party (see the state getter for more information)
   * @param {Object} [finalStatement] - the complete final statement
   * @param {Uint8Array} [previous] - a link to the previous pop-party, if available
   * @param {Uint8Array} [next] - a link to the next pop-party, if available
   * @param {Uint8Array} [service] - the public key of the service, if available
   */
  constructor(bc, instanceId, instance, state, finalStatement, previous, next, service) {
    this._bc = bc;
    this._instanceId = instanceId;
    this._instance = instance;
    this._state = state;
    this._finalStatement = finalStatement;
    this._previous = previous;
    this._next = next;
    this._service = service;
  }

  /**
   * Return the state of the party :
   * 1: it is a configuration reply
   * 2: it is a finalized pop party
   *
   * @return {number}
   */
  get state() {
    return this._state;
  }

  /**
   * @return {Object} - the literal object decoded by Protobuf
   */
  get finalStatement() {
    return this._finalStatement;
  }

  /**
   * Creates a new PopPartyInstance from an instance ID and try to contact the
   * byzcoin to get the last data
   *
   * @param {ByzCoinRPC} bc - the byzcoin instance
   * @param {Uint8Array} instanceId - the contract instance id
   */
  static fromInstanceId(bc, instanceId) {
    return new PopPartyInstance(bc, instanceId).update();
  }

  /**
   * Store the final statement on the ledger. This happens after the
   * party description has been published an the party finalized
   *
   * @param {Object} finalStatement - the final statement
   * @param {Signer} signer - one of the organizer of the party
   * @return {Promise}
   */
  storeFinalStatement(finalStatement, signer) {
    const model = root.lookup("FinalStatement");
    const message = model.create(finalStatement);
    const marshal = model.encode(message).finish();
    const invoke = Invoke.fromArgumentInfo(
      "Finalize",
      "FinalStatement",
      marshal
    );
    const inst = Instruction.createInvokeInstruction(
      this._instanceId,
      new Uint8Array(32),
      0,
      1,
      invoke
    );
    inst.signBy(this._instance.darcId, [signer]);
    const clientTransaction = new ClientTransaction([inst]);

    return this._bc.sendTransactionAndWait(clientTransaction, 10);
  }

  /**
   * Contact the ledger to try getting the last data
   *
   * @return {Promise<PopPartyInstance>}
   */
  update() {
    return this._bc.getProof(this._instanceId).then(proof => {
      this._instance = Instance.fromProof(proof);
      const model = root.lookup("PopPartyInstance");
      const protoObject = model.decode(this._instance.data);

      this._state = protoObject.state;
      this._finalStatement = protoObject.finalstatement;
      this._previous = protoObject.previous;
      this._next = protoObject.next;
      this._service = protoObject.service;

      return Promise.resolve(this);
    });
  }

  /**
   * After that the party has been finalized, each attendee receive a certain
   * amount of coin on a personnal account. This method compute the instance
   * id of this account, depending on the public key of the attendee
   *
   * @param {Point} Pub - the public key of the attendee
   * @return {Uint8Array} - the coin instance id of the attendee
   */
  getAccountInstanceId(pub) {
    const hash = crypto.createHash("sha256");
    hash.update(this._instanceId);
    hash.update(pub);

    let b = hash.digest();
    return new Uint8Array(b.buffer, b.byteOffset, b.byteLength / Uint8Array.BYTES_PER_ELEMENT);
  }

  getServiceCoinInstanceId(){
    return this.getAccountInstanceId(this._service);
  }
}

module.exports = PopPartyInstance;

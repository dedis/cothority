const root = require("../../protobuf").root;
const Instance = require("../Instance");
const Invoke = require("../Invoke");
const Instruction = require("../Instruction");
const ClientTransaction = require("../ClientTransaction");

class PopPartyInstance {
  /**
   * @param {OmniledgerRPC} ol
   * @param {Uint8Array} instanceId
   * @param {Instance} [instance]
   * @param {number} [state]
   * @param {Object} [finalStatement]
   * @param {Uint8Array} [previous]
   * @param {Uint8Array} [next]
   */
  constructor(ol, instanceId, instance, state, finalStatement, previous, next) {
    this._ol = ol;
    this._instanceId = instanceId;
    this._instance = instance;
    this._state = state;
    this._finalStatement = finalStatement;
    this._previous = previous;
    this._next = next;
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
   * @return {Object} - the literral object from decoded by Protobuf
   */
  get finalStatement() {
    return this._finalStatement;
  }

  /**
   * @param {OmniledgerRPC} ol
   * @param {Uint8Array} instanceId
   */
  static fromInstanceId(ol, instanceId) {
    return new PopPartyInstance(ol, instanceId).update();
  }

  /**
   *
   * @param {Object} finalStatement
   * @param {Signer} signer
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
      new Uint8Array(),
      0,
      1,
      invoke
    );
    inst.signBy(this._instance.darcId, [signer]);
    const clientTransaction = new ClientTransaction([inst]);

    return this._ol.sendTransactionAndWait(clientTransaction, 10);
  }

  /**
   * @return {Promise<PopPartyInstance>}
   */
  update() {
    return this._ol.getProof(this._instanceId).then(proof => {
      this._instance = Instance.fromProof(proof);
      const model = root.lookup("PopPartyInstance");
      const protoObject = model.decode(this._instance.data);

      this._state = protoObject.state;
      this._finalStatement = protoObject.finalstatement;
      this._previous = protoObject.previous;
      this._next = protoObject.next;

      return Promise.resolve(this);
    });
  }
}

module.exports = PopPartyInstance;

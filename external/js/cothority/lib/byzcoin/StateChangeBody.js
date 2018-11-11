const root = require("../protobuf/index.js").root;

/**
 * StateChangeBody represents the data that is stored in a leaf node in the trie.
 */
class StateChangeBody {
  /**
   * Creates a StateChangeBody.
   */
  constructor(stateaction, contractid, darcid, value) {
    this._stateAction = stateaction;
    this._contractID = contractid;
    this._darcID = darcid;
    this._value = value;
  }

  /**
   * Constructs a StateChangeBody from its protobuf representation.
   * @param {Uint8Array} buf - protobuf encoded byte array
   */
  static fromByteBuffer(buf) {
    if (!(buf instanceof Uint8Array)) {
      throw "buf must be of type Uint8Array in StateChangeBody";
    }
    const model = root.lookup("StateChangeBody");
    let body = model.decode(buf);
    return new StateChangeBody(body.stateaction, body.contractid, body.darcid, body.value);
  }

  /**
   * Getter for stateAction
   */
  get stateAction() {
    return this._stateAction;
  }

  /**
   * Getter for contract ID
   */
  get contractID() {
    return this._contractID;
  }

  /**
   * Getter for darc ID
   */
  get darcID() {
    return this._darcID;
  }

  /**
   * Getter for the value stored in the instance
   */
  get value() {
    return this._value;
  }
}

module.exports = StateChangeBody;
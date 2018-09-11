class Signature {
  /**
   *
   * @param {Uint8Array} [signature]
   * @param {Identity} signer
   */
  constructor(signature, signer) {
    this._signature = signature;
    this._signer = signer;
  }

  /**
   * @return {Identity}
   */
  get signer() {
    return this._signer;
  }

  /**
   * @return {Uint8Array}
   */
  get signature() {
    return this._signature;
  }

  /**
   * Create an object with all the necessary field needed to be a valid message
   * in the sense of protobufjs. This object can then be used with the "create"
   * method of protobuf
   *
   * @return {Object}
   */
  toProtobufValidMessage() {
    return {
      signature: this._signature,
      signer: this._signer.toProtobufValidMessage()
    };
  }
}

module.exports = Signature;

class Argument {
  /**
   * Argument is used in an Instruction. It will end up as the input argument
   * of the OmniLedger smart contract.
   * @param {string} name - The name of the argument
   * @param {Uint8Array} value - The value of the argument
   */
  constructor(name, value) {
    this._name = name;
    this._value = value.slice(0);
  }

  /**
   * Getter for the name
   * @return {string}
   */
  get name() {
    return this._name;
  }

  /**
   * Getter for the value
   * @return {Uint8Array}
   */
  get value() {
    return this._value.slice(0);
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
      name: this.name,
      value: this.value
    };
  }
}

module.exports = Argument;

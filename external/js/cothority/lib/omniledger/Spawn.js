/**
 * Spawn is an operation that an Instruction can take, it should be used for creating an object.
 */
class Spawn {
  /**
   * Constructor for the spawn action.

   * @param {string} contractId - The contract ID
   * @param {Argument[]} argumentsList - The initial arguments for running the contract.
   */
  constructor(contractId, argumentsList) {
    this._contractId = contractId;
    this._arguments = argumentsList;
  }

  /**
   * @return {string} - The contract ID
   */
  get contractId() {
    return this._contractId;
  }

  /**
   * @return {Argument[]} - The arguments
   */
  get argumentsList() {
    return this._arguments;
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
      contractid: this._contractId,
      args: this._arguments.map(arg => arg.toProtobufValidMessage())
    };
  }
}

module.exports = Spawn;

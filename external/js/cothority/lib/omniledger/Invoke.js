const Argument = require("./Argument");

/**
 * Invoke is an operation that an Instruction can take, it should be used
 * for mutating an object.
 */
class Invoke {
  /**
   * Creates an new invoke action
   *
   * @param {string} command - the command to invoke in the contract
   * @param {Array<Argument>} argumentsList - the arguments for the contract
   */
  constructor(command, argumentsList) {
    this._command = command;
    this._arguments = argumentsList;
  }

  /**
   * Creates a new Invoke command from one Argument
   *
   * @param {string} command - the command to invoke in the contract
   * @param {string} argument - name of the argument
   * @param {Uint8Array} value - the value of the argument
   * @return {Invoke}
   */
  static fromArgumentInfo(command, argument, value) {
    return new Invoke(command, [new Argument(argument, value)]);
  }

  /**
   * Getter for the command
   *
   * @return {string}
   */
  get command() {
    return this._command;
  }

  /**
   * Getter for the arguments
   *
   * @return {Array<Argument>}
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
      command: this._command,
      args: this.argumentsList.map(arg => arg.toProtobufValidMessage())
    };
  }
}

module.exports = Invoke;

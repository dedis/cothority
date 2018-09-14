/**
 * ClientTransaction is a set of instructions are will be executed atomically
 * by ByzCoin.
 */
class ClientTransaction {
  /**
   * @param {Array<Instruction>} instructions - The list of instruction that
   * should be executed atomically.
   */
  constructor(instructions) {
    this._instructions = instructions;
  }

  /**
   * Getter for the instruction
   * @return {Array<Instruction>}
   */
  get instructions() {
    return this._instructions;
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
      instructions: this._instructions.map(inst =>
        inst.toProtobufValidMessage()
      )
    };
  }
}

module.exports = ClientTransaction;

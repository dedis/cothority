/**
 * Delete is an operation that an Instruction can take, it should be used for deleting an object.
 */
class Delete {
  constructor() {}

  /**
   * Create an object with all the necessary field needed to be a valid message
   * in the sense of protobufjs. This object can then be used with the "create"
   * method of protobuf
   *
   * @return {Object}
   */
  toProtobufValidMessage() {
    return {};
  }
}

module.exports = Delete;

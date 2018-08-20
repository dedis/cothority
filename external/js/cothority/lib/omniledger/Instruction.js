/**
 * An instruction is sent and executed by OmniLedger.
 */
class Instruction {
  /**
   * Cunstruct an instruction when the complete configuration is known
   *
   * @param {Uint8Array} [instanceId] - The ID of the object, which must be unique
   * @param {Uint8Array} nonce - the nonce of the object
   * @param {number} index - the index of the instruction in the atomic set
   * @param {number} length - the length of the atomic set
   * @param {Spawn} [spawnInst] - the spawn object, which contains the value and the argument
   * @param {Invoke} [invokeInst] - the invoke object
   * @param {Delete} [deleteInst] - the delete object
   * @param {Array} [signatures] - the list of signatures
   */
  constructor(
    instanceId,
    nonce,
    index,
    length,
    spawnInst,
    invokeInst,
    deleteInst,
    signatures
  ) {
    this._instanceId = instanceId;
    this._nonce = nonce;
    this._index = index;
    this._length = length;
    this._spawnInst = spawnInst;
    this._invokeInst = invokeInst;
    this._deleteInst = deleteInst;
    this._signatures = signatures;
  }

  /**
   * Use this constructor if it is a spawn instruction, i.e. you want to create a new object.
   *
   * @see {@link Instruction} refer to the constructor
   * @param {Uint8Array} nonce
   * @param {number} index
   * @param {number} length
   * @param {Spawn} spawnInst
   * @return {Instruction}
   */
  static createSpawnInstruction(nonce, index, length, spawnInst) {
    return new Instruction(
      undefined,
      nonce,
      index,
      length,
      spawnInst,
      undefined,
      undefined,
      undefined
    );
  }

  /**
   * Use this constructor if it is a spawn instruction, i.e. you want to create a new object.
   *
   * @see {@link Instruction} refer to the constructor
   * @param {Uint8Array} instanceId
   * @param {Uint8Array} nonce
   * @param {number} index
   * @param {number} length
   * @param {Invoke} invokeInst
   * @return {Instruction}
   */
  static createInvokeInstruction(instanceId, nonce, index, length, invokeInst) {
    return new Instruction(
      instanceId,
      nonce,
      index,
      length,
      undefined,
      invokeInst,
      undefined,
      undefined
    );
  }

  /**
   * Use this constructor if it is a delete instruction, i.e. you want to delete an object.
   *
   * @see {@link Instruction} refer to the constructor
   * @param instanceId
   * @param nonce
   * @param index
   * @param length
   * @param deleteInst
   * @return {Instruction}
   */
  static createDeleteInstruction(instanceId, nonce, index, length, deleteInst) {
    return new Instruction(
      instanceId,
      nonce,
      index,
      length,
      undefined,
      undefined,
      deleteInst,
      undefined
    );
  }

  /**
   * Getter for the instance ID
   *
   * @return {Uint8Array}
   */
  get instanceId() {
    return this._instanceId;
  }

  /**
   * Set the signatures for this instruction
   *
   * @param sig
   */
  set signatures(sig) {
    this._signatures = sig.slice(0);
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
      instanceid: this._instanceId,
      nonce: this._nonce,
      index: this._index,
      length: this._length,
      spawn: this._spawnInst,
      invoke: this._invokeInst,
      delete: this._deleteInst,
      signatures: this._signatures
    };
  }
}

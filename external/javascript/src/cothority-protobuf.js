import Root from './models/root'

/**
 * Base class for the protobuf library that provides helpers to encode and decode
 * messages according to a given model
 *
 * @author Gaylor Bosson (gaylor.bosson@epfl.ch)
 */
export default class CothorityProtobuf {

  /**
   * @constructor
   */
  constructor() {
    this.root = Root;
  }
  
  /**
   * Encode a model to be transmitted over websocket
   * @param {String} name
   * @param {Object} fields
   * @returns {*|Buffer|Uint8Array}
   */
  encodeMessage(name, fields) {
    const model = this.getModel(name);
    
    // Create the message with the model
    const msg = model.create(fields);

    // Encode the message in a BufferArray
    return model.encode(msg).finish();
  }
  
  /**
   * Decode a message coming from a websocket
   * @param {String} name
   * @param {*|Buffer|Uint8Array} buffer
   */
  decodeMessage(name, buffer) {
    const model = this.getModel(name);
    return model.decode(buffer);
  }
  
  /**
   * Return the protobuf loaded model
   * @param {String} name
   * @returns {ReflectionObject|?ReflectionObject|string}
   */
  getModel(name) {
    return this.root.lookup(`${name}`);
  }
}

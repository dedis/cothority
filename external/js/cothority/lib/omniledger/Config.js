const root = require("../protobuf/index.js").root;

/**
 * Config is the genesis configuration of an omniledger instance. It can be stored only once in omniledger
 * and defines the basic running parameters of omniledger.
 */
class Config {
  /**
   * Creates a config from knwon informations
   * @param {number} blockInterval
   */
  constructor(blockInterval) {
    this._blockInterval = blockInterval;
  }

  /**
   * @return {number} - the blockinterval used
   */
  get blockInterval() {
    return this._blockInterval;
  }

  /**
   * Creates a Config from a byte array
   * @param {Uint8Array} buf
   * @return {Config}
   */
  static fromByteBuffer(buf) {
    if (!(buf instanceof Uint8Array)) {
      throw "buf must be of type UInt8Array";
    }
    const configModel = root.lookup("ChainConfig");
    let config = configModel.decode(buf);

    return new Config(config.blockinterval);
  }

  /**
   * Check if two Configs are equal
   * @param {Object} config
   * @return {boolean} - true if the config are equals (e.i if they have
   * the same blockinterval)
   */
  equals(config) {
    if (config === undefined) return false;
    if (!(config instanceof Config)) return false;
    return this._blockInterval === config.blockInterval;
  }
}

module.exports = Config;

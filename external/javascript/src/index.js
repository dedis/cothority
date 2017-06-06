import CothorityProtobuf from './cothority-protobuf'

/**
 * Helpers to encode and decode messages of the Cothority
 *
 * @author Gaylor Bosson (gaylor.bosson@epfl.ch)
 */
class CothorityMessages extends CothorityProtobuf {
  
  /**
   * Create an encoded message to make a sign request to a cothority node
   * @param {Uint8Array} message - Message to sign stored in a Uint8Array
   * @param {Array} servers - list of ServerIdentity
   * @returns {*|Buffer|Uint8Array}
   */
  createSignatureRequest(message, servers) {
    if (!(message instanceof Uint8Array)) {
      throw new Error("message must be a instance of Uint8Array");
    }
    
    const fields = {
      message,
      roster: {
        list: servers
      }
    };
    
    return this.encodeMessage('SignatureRequest', fields);
  }
  
  /**
   * Return the decoded response of a signature request
   * @param {*|Buffer|Uint8Array} response - Response of the Cothority
   * @returns {Object}
   */
  decodeSignatureResponse(response) {
    response = new Uint8Array(response);

    return this.decodeMessage('SignatureResponse', response);
  }
  
  /**
   * Return the decoded response of a status request
   * @param {*|Buffer|Uint8Array} response - Response of the Cothority
   * @returns {*}
   */
  decodeStatusResponse(response) {
    response = new Uint8Array(response);

    return this.decodeMessage('StatusResponse', response);
  }

  /**
   * Create a message to store a new block
   * @param {Uint8Array} id - ID of the current latest block
   * @param {Array} servers - list of ServerIdentity
   * @returns {*|Buffer|Uint8Array}
   */
  createStoreSkipBlockRequest(id, servers) {
    if (!(id instanceof Uint8Array)) {
      throw new Error("message must be a instance of Uint8Array");
    }

    return this.encodeMessage('StoreSkipBlockRequest', {
      LatestID: id,
      NewBlock: {
        MaximumHeight: 1,
        BaseHeight: 1,
        Data: new Uint8Array([]),
        Roster: {
          list: servers
        }
      }
    });
  }

  /**
   * Return the decoded message of a store skip block request
   * @param {*|Buffer|Uint8Array} response - Response of the Cothority
   * @returns {*}
   */
  decodeStoreSkipBlockResponse(response) {
    response = new Uint8Array(response);

    return this.decodeMessage('StoreSkipBlockResponse', response);
  }

  /**
   * Create a message request to get the latest blocks of a skip-chain
   * @param {Uint8Array} id - ID of the genesis block of the skip-chain
   * @returns {*|Buffer|Uint8Array}
   */
  createLatestBlockRequest(id) {
    if (!(id instanceof Uint8Array)) {
      throw new Error("message must be a instance of Uint8Array");
    }

    return this.encodeMessage('LatestBlockRequest', {
      LatestID: id
    });
  }

  /**
   * Return the decoded message of a latest block request
   * @param {*|Buffer|Uint8Array} response - Response of the Cothority
   * @returns {*}
   */
  decodeLatestBlockResponse(response) {
    response = new Uint8Array(response);

    return this.decodeMessage('LatestBlockResponse', response);
  }

  /**
   * Create a message request to get a random number
   * @returns {*|Buffer|Uint8Array}
   */
  createRandomMessage() {
    return this.encodeMessage('RandomRequest');
  }

  /**
   * Return the decoded message of a random number request
   * @param {*|Buffer|Uint8Array} response - Response of the Cothority
   * @returns {*}
   */
  decodeRandomResponse(response) {
    response = new Uint8Array(response);

    return this.decodeMessage('RandomResponse', response);
  }
  
}

/**
 * Singleton
 */
export default new CothorityMessages();
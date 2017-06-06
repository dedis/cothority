import CP from './cothority-protobuf'
import Faker from 'faker'
import ByteBuffer from 'bytebuffer'

const CothorityProtobuf = new CP();

const mockSignReq = "CiCAHxMpFWVDC2nQwYfKiXWtrOGwZRS0Aw9T/ZOkwK3ZwxJgEl4KIOXiPlhTmgnTIR2PoPs0ddSGVeDAbYPpPI5uf" +
  "Raqh8EGEhCyCNMGs0dR/Luz1OnIZYjZGhd0Y3A6Ly83OC40Ni4yMjcuNjA6Nzc3MCIPSW5laXRpJ3Mgc2VydmVy";

describe('Protobuf', () => {

  it('should encode and decode correctly', () => {
    const encoded = CothorityProtobuf.encodeMessage('StatusResponse', {
      system: {
        status1: {
          field: {
            field1: 'success'
          }
        }
      },
      server: {
        address: Faker.internet.ip(),
        description: Faker.lorem.sentence(),
        public: Uint8Array.from([1, 1, 1, 1]),
        id: Uint8Array.from([2, 2, 2, 2])
      }
    });

    const decoded = CothorityProtobuf.decodeMessage('StatusResponse', encoded);

    expect(decoded.server.public).toBeDefined();
    expect(decoded.system.status1.field.field1).toBe('success');
  });

  it('should decode and encode in the same way', () => {
    const buffer = Uint8Array.from(atob(mockSignReq), c => c.charCodeAt(0));

    const decoded = CothorityProtobuf.decodeMessage('SignatureRequest', buffer);
    const encoded = CothorityProtobuf.encodeMessage('SignatureRequest', decoded);

    expect(ByteBuffer.wrap(encoded).toBase64()).toBe(mockSignReq);
  });

});

import com.google.protobuf.ByteString;
import proto.OCSProto;

import java.security.KeyPair;
import java.security.PrivateKey;
import java.security.PublicKey;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;

public class Document {
    public byte[] id;
    public byte[] data;
    public byte[] symmetricKey;
    public byte[] darc_id;

    public Document(byte[] data) {
        this.data = data;
        symmetricKey = Crypto.uuid4();
    }

    public Document(String data) {
        this(data.getBytes());
    }

    public OCSProto.OCSWrite getWrite(PublicKey X) throws Crypto.CryptoException {
        OCSProto.OCSWrite.Builder write = OCSProto.OCSWrite.newBuilder();
//        cipher := network.Suite.Cipher(symKey)
//        encData := cipher.Seal(nil, data)
        write.setData(ByteString.copyFrom(data));

        // Input:
//   - suite - the cryptographic suite to use
//   - X - the aggregate public key of the DKG
//   - key - the symmetric key for the document
//
// Output:
//   - U - the schnorr commit
//   - Cs - encrypted key-slices
//        func EncodeKey(suite abstract.Suite, X abstract.Point, key []byte) (U abstract.Point, Cs []abstract.Point) {
//            r := suite.Scalar().Pick(random.Stream)
//            U = suite.Point().Mul(nil, r)
//
//            rem := make([]byte, len(key))
//            copy(rem, key)
//            for len(rem) > 0 {
//                var kp abstract.Point
//                kp, rem = suite.Point().Pick(rem, random.Stream)
//                C := suite.Point().Mul(X, r)
//                Cs = append(Cs, C.Add(C, kp))
//            }
//            return
//        }
        KeyPair randkp = Crypto.keyPair();
        PrivateKey r = randkp.getPrivate();
        PublicKey U = randkp.getPublic();
//        PublicKey C = Crypto.toGroup(X).scalarMultiply(Crypto.toScalar(r));

        List<PublicKey> Cs = new ArrayList<>();
        for (int i = 0; i < symmetricKey.length; i += Crypto.pubLen) {
            int to = i + Crypto.pubLen;
            if (to > symmetricKey.length) {
                to = symmetricKey.length - i;
            }
            PublicKey kp = Crypto.pubStore(Arrays.copyOfRange(symmetricKey, i, to));
            Cs.add(kp);
        }

        write.setU(Crypto.toProto(U));
        return write.build();
    }
}

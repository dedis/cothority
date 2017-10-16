package ch.epfl.dedis.ocs;


import javax.annotation.Nonnull;
import java.security.SignatureException;
import java.util.UUID;

/**
 * Representation of EPFL skipchain user.
 */
public interface User {
    /**
     * Return UUID which is immutable ID of skipchain user.
     * @return ID of user
     */
    @Nonnull
    UUID getUserId();

    /**
     * Sign request. Once user would like to authorize some operation in skipchain it is required to sign transaction
     * (request) which will be send to the skipchain.
     * @param signRequest transaction (request) which is about to send to skipchain
     * @return signature of a request
     * @throws SignatureException can be thrown in case of internal processing problems and also when user decide to
     * reject request.
     */
    @Nonnull
    UserSignature sign(@Nonnull byte[] signRequest) throws SignatureException;

}

package ch.epfl.dedis.ocs;

import javax.annotation.Nonnull;
import javax.annotation.Nullable;

public interface Document20 {
    /**
     * Return document ID
     * @return document ID
     */
    @Nonnull
    DocumentId getDocumentId();

    /**
     * Name of document
     * @return name of document
     */
    @Nonnull
    String getName();

    /**
     * Document description (should describe content of document)
     * @return document description
     */
    @Nonnull
    String getDescription();

    /**
     * Return decrypted key material.
     * // TODO: it is possible that in the future key material returned from EPFL library will
     * be returned encrypted in asymmetric way so only eligible user will be able to decrypt and use key material
     *
     * @return key material or null in case of listing operations
     */
    @Nullable
    byte[] getKeyMaterial();

    /**
     * Returns document body. It is returned in the same way as it was passed during storing document.
     *
     * @return document body or null in case of listing operations
     */
    @Nullable
    byte[] getEncryptedBody();
}

package ch.epfl.dedis.lib.exception;

public class CothorityAlreadyExistsException extends CothorityException {
    public CothorityAlreadyExistsException() {
    }
    public CothorityAlreadyExistsException(String message) {
        super(message);
    }
}

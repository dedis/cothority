public class Document {
    private byte[] ID;
    private byte[] symmetricKey;
    private byte[] document;

    public Document(byte[] id){
        this.ID = id;
    }
}

package ch.epfl.dedis.lib.eventlog;

public class Event {
    private long when; // in nano seconds
    private String topic;
    private String content;

    public Event(long when, String topic, String content) {
        this.when = when;
        this.topic = topic;
        this.content = content;
    }

    public Event(String topic, String content) {
        this(System.currentTimeMillis() * 1000 * 1000, topic, content);
    }
}

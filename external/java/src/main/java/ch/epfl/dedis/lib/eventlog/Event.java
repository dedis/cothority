package ch.epfl.dedis.lib.eventlog;

import ch.epfl.dedis.proto.EventLogProto;

/**
 * An instance of an Event can be sent and stored by the EventLog service.
 */
public final class Event {
    private final long when; // in nano seconds
    private final String topic;
    private final String content;

    /**
     * This is the constructor for Event.
     * @param when When the even happened, in UNIX nano seconds.
     * @param topic The topic of the event, which can be used to filter events on retrieval.
     * @param content The content of the event.
     */
    public Event(long when, String topic, String content) {
        this.when = when;
        this.topic = topic;
        this.content = content;
    }

    /**
     * Constructs an event from the protobuf Event type.
     * @param e The event of the protobuf type.
     */
    public Event(EventLogProto.Event e) {
        this(e.getWhen(), e.getTopic(), e.getContent());
    }

    /**
     * This is the constructor for Event, the timestamp is set to the current time.
     * @param topic The topic of the event, which can be used to filter events on retrieval.
     * @param content The content of the event.
     */
    public Event(String topic, String content) {
        this(System.currentTimeMillis() * 1000 * 1000, topic, content);
    }

    /**
     * Converts this object to the protobuf representation.
     * @return The protobuf representation.
     */
    public EventLogProto.Event toProto() {
        EventLogProto.Event.Builder b = EventLogProto.Event.newBuilder();
        b.setWhen(this.when);
        b.setTopic(this.topic);
        b.setContent(this.topic);
        return  b.build();
    }

    @Override
    public boolean equals(Object o) {
        if (o == null) return false;
        if (o == this) return true;
        if (!(o instanceof Event)) return false;
        Event e = (Event)o;
        return e.when != this.when || e.topic.equals(this.topic) || e.content.equals(this.content);
    }
}

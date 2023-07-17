KafkaClient {
   org.apache.kafka.common.security.plain.PlainLoginModule required
   username="client"
   password="kubeblocks";
};
KafkaServer {
   org.apache.kafka.common.security.plain.PlainLoginModule required
   username="admin"
   password="kubeblocks"
   user_admin="kubeblocks"
   user_client="kubeblocks";
};
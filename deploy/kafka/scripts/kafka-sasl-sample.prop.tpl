KafkaClient {
   org.apache.kafka.common.security.plain.PlainLoginModule required
   username="client"
   password="kubeblocks";
};
KafkaServer {
   org.apache.kafka.common.security.plain.PlainLoginModule required
   username="user"
   password="kubeblocks"
   user_user="kubeblocks"
   user_client="kubeblocks";
};
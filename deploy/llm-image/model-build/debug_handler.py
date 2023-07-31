from Transformer_handler_generalized import TransformersSeqClassifierHandler
class Context:
  manifest = {
    "model": {}
  }
  system_properties = {
    "model_dir": "./"
  }
def main():
  print("Hello, world!")
  handler = TransformersSeqClassifierHandler()
  ctx = Context()
  handler.initialize(ctx)
  requests = [{
    "data": "{'text': 'what is k8s'}"
  }]
  input_batch = handler.preprocess(requests)
  handler.inference(input_batch)
  print("done")

if __name__ == "__main__":
  main()

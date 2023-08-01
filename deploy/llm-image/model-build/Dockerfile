FROM pytorch/torchserve as torchserve
COPY ./pytorch_model.bin .
COPY ./config.json .
COPY ./Transformer_handler_generalized.py .
COPY ./setup_config.json .
COPY ./requirements.txt .
RUN torch-model-archiver --model-name gpt2 --version 1.0 --handler ./Transformer_handler_generalized.py --extra-files "pytorch_model.bin,config.json,./setup_config.json" -r "./requirements.txt" --export-path /tmp

FROM alpine:edge
COPY --from=torchserve /tmp/gpt2.mar /model_store/
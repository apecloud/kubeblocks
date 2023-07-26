FROM pytorch/torchserve as torchserve
COPY ./gpt2/pytorch_model.bin .
COPY ./gpt2/config.json .
COPY ./Transformer_handler_generalized.py .
COPY ./setup_config.json .
COPY ./requirements.txt .
RUN pip install -r requirements.txt
RUN torch-model-archiver --model-name gpt2 --version 1.0 --handler ./Transformer_handler_generalized.py --extra-files "pytorch_model.bin,config.json,./setup_config.json" --export-path /tmp

FROM alpine:edge
COPY --from=torchserve /tmp/gpt2.mar /model_store/
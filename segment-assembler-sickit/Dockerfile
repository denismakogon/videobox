FROM denismakogon/opencv3-slim:edge

RUN apt-get update && apt-get install -qy ffmpeg
ADD requirements.txt /function/
WORKDIR /function/
RUN pip install --no-cache --no-cache-dir --upgrade -r requirements.txt
ADD . /function/
RUN rm -fr ~/.cache/pip /tmp* requirements.txt func.yaml Dockerfile
ENTRYPOINT ["python", "/function/func.py"]

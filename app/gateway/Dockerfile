# 网关镜像：官方 APISIX + 自定义 Lua 插件 + standalone 配置 / 路由。
#
#   docker build -t <registry>/gateway:<tag> app/gateway
#
# 运行时只需注入环境变量（见 conf/apisix.yaml 顶部），不挂载任何文件。
# 基础镜像可用 --build-arg APISIX_IMAGE=... 替换。
ARG APISIX_IMAGE=apache/apisix:3.14.1-debian
FROM ${APISIX_IMAGE}

# 自定义插件：按 require 路径放到 extra_lua_path（conf/config.yaml）下，不覆盖内置插件
COPY --chown=apisix:apisix apisix/ /usr/local/apisix/custom/apisix/
# 数据面配置 + 路由（standalone 模式，启动时读取并替换 ${{VAR}}）
COPY --chown=apisix:apisix conf/config.yaml conf/apisix.yaml /usr/local/apisix/conf/

EXPOSE 9080

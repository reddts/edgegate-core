include $(TOPDIR)/rules.mk

PKG_NAME:=edgegate-core
PKG_VERSION:=1.0.5
PKG_RELEASE:=1

PKG_SOURCE:=$(PKG_NAME)-$(PKG_VERSION).tar.gz
PKG_SOURCE_URL:=https://codeload.github.com/reddts/edgegate-core/tar.gz/v$(PKG_VERSION)?
PKG_HASH:=3ec509335ee44a021b41e18ce3fd030f637f4cfeef4882d0e763102016571c96

PKG_LICENSE:=https://github.com/reddts/edgegate-core/edit/main/LICENSE.md
PKG_LICENSE_FILES:=LICENSE
PKG_MAINTAINER:=Edgegate <wrt@edgegate.com>

PKG_BUILD_DEPENDS:=golang/host
PKG_BUILD_PARALLEL:=1
PKG_BUILD_FLAGS:=no-mips16

GO_PKG:=github.com/reddts/edgegate-core
GO_PKG_BUILD_PKG:=$(GO_PKG)/cli

GO_PKG_LDFLAGS_X:=$(GO_PKG)/constant.Version=$(PKG_VERSION)

include $(INCLUDE_DIR)/package.mk
include ../../lang/golang/golang-package.mk

define Package/edgegate-core
  TITLE:=Edgegate universal proxy platform
  SECTION:=net
  CATEGORY:=Network
  URL:=https://www.grde.net/
  DEPENDS:=$(GO_ARCH_DEPENDS) +ca-bundle +kmod-inet-diag +kmod-tun
  USERID:=edgegate=15566:edgegate=15566
endef

define Package/edgegate-core/description
  Edgegate is a universal proxy platform which supports hysteria, SOCKS, Shadowsocks,
  ShadowTLS, Tor, trojan, VLess, VMess, WireGuard and so on.
endef

GO_PKG_TAGS:=with_gvisor,with_quic,with_wireguard,with_ech,with_utls,with_clash_api,with_grpc,with_low_memory,with_conntrack

define Package/edgegate-core/conffiles
/etc/config/edgegate
/etc/edgegate/
endef

define Package/edgegate-core/install

	$(INSTALL_DIR) $(1)/usr/bin/
	$(INSTALL_BIN) $(GO_PKG_BUILD_BIN_DIR)/cli $(1)/usr/bin/edgegate-cli

	$(INSTALL_DIR) $(1)/etc/edgegate
	$(INSTALL_DATA) ./files/edgegate.json $(1)/etc/edgegate/appconfig.json

	$(INSTALL_DIR) $(1)/etc/config/
	$(INSTALL_CONF) ./files/edgegate.conf $(1)/etc/config/edgegate

	$(INSTALL_DIR) $(1)/etc/init.d/
	$(INSTALL_BIN) ./files/edgegate.init $(1)/etc/init.d/edgegate

	$(CP) ./files/webui $(1)/usr/bin/
endef

$(eval $(call BuildPackage,edgegate-core))

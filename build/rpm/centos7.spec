# build with the following command:
# rpmbuild -bb
%define debug_package %{nil}

Name:       tg-toolset-golang
Version:    %{getenv:VERSION}
Release:    1%{?dist}
Summary:    CLI tools for interacting various services managed by the TG.
License:    FIXME
URL: https://github.com/Donders-Institute/%{name}
Source0: https://github.com/Donders-Institute/%{name}/archive/%{version}.tar.gz

BuildArch: x86_64
Requires: libcap acl nfs4-acl-tools

# defin the GOPATH that is created later within the extracted source code.
%define gopath %{_tmppath}/go.rpmbuild-%{name}-%{version}

%description
CLI tools for interacting various services managed by the TG.

%prep
%setup -q

%build
# create GOPATH structure within the source code
mkdir -p %{gopath}
# copy entire directory into gopath, this duplicate the source code
GOPATH=%{gopath} make

%install
mkdir -p %{buildroot}/%{_bindir}
mkdir -p %{buildroot}/%{_sbindir}
## install files for client tools
install -m 755 %{gopath}/bin/pdbutil %{buildroot}/%{_sbindir}/pdbutil
install -m 755 %{gopath}/bin/lab_bookings %{buildroot}/%{_sbindir}/lab_bookings
install -m 755 %{gopath}/bin/pacs_getstudies %{buildroot}/%{_sbindir}/pacs_getstudies
install -m 755 %{gopath}/bin/pacs_streamdata %{buildroot}/%{_sbindir}/pacs_streamdata
install -m 755 %{gopath}/bin/prj_setacl %{buildroot}/%{_bindir}/prj_setacl
install -m 755 %{gopath}/bin/prj_getacl %{buildroot}/%{_bindir}/prj_getacl
install -m 755 %{gopath}/bin/prj_delacl %{buildroot}/%{_bindir}/prj_delacl
install -m 755 %{gopath}/bin/prj_chown  %{buildroot}/%{_bindir}/prj_chown

%files
%{_sbindir}/pdbutil
%{_sbindir}/lab_bookings
%{_sbindir}/pacs_getstudies
%{_sbindir}/pacs_streamdata
%{_bindir}/prj_setacl
%{_bindir}/prj_getacl
%{_bindir}/prj_delacl
%{_bindir}/prj_chown

%post
echo "setting linux capabilities for ACL utilities ..."
setcap cap_fowner,cap_sys_admin+eip %{_bindir}/prj_delacl
setcap cap_fowner,cap_sys_admin+eip %{_bindir}/prj_setacl
setcap cap_sys_admin+eip %{_bindir}/prj_getacl
setcap cap_chown+eip %{_bindir}/prj_chown

%clean
chmod -R +w %{gopath}
rm -rf %{gopath}
rm -f %{_topdir}/SOURCES/%{version}.tar.gz
rm -rf $RPM_BUILD_ROOT

%changelog
* Thu Jan 27 2022 Hong Lee <h.lee@donders.ru.nl>
- remove 'repoadm' and 'repcli'
* Wed Dec 02 2020 Hong Lee <h.lee@donders.ru.nl>
- introduced 'prj_chown' cli
* Tue Jul 21 2020 Hong Lee <h.lee@donders.ru.nl> - 0.2
- renamed 'repoutil' to 'repoadm'
- added 'repocli'
* Thu Jun 23 2020 Hong Lee <h.lee@donders.ru.nl> - 0.1
- first rpmbuild implementation

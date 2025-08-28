package main

var (
	OpeningGreeting = `Selamat datang di Asisten Virtual PPPK GKJP:

1. Info Pemilihan
2. Registrasi pemilihan Online
3. Kirim ulang surat suara Digital

Balas dengan angka: 1, 2 atau 3.`

	ErrorSaving = `Maaf, terjadi kesalahan saat menyimpan data. Silakan coba lagi.`

	NotRegistered = `Nomor anda belum terdaftar
Silahkan menghubungi Majelis Wilayah atau Panitia`

	ErrorQuerying = `Maaf, terjadi kesalahan saat memeriksa data. Silakan coba lagi.`
	ErrorWilayah  = `Wilayah pelayanan tidak valid. Silahkan isi pp1/pp2/serpong/bukit/reni`
	ErrorDOB      = `Tahun lahir tidak valid. Silahkan isi tahun lahir 4 digit`

	SuccessRegister = `Terima kasih!

Pendaftaran pemilihan online berhasil dengan data sebagai berikut:

Nama: %s
Wilayah: %s
Tahun Lahir: %s
Surat suara Digital: https://pemilihan.gkj-pamulang.org/%s

Simpan surat suara digital ini dengan aman.`

	RegistrationHelp = `%sUntuk mendaftar pemilihan online, silahkan kirim pesan dengan format:

DAFTAR-<nama lengkap jemaat>-<wilayah pelayanan>-<tahun lahir>#

Tahun Lahir 4 Digit, Contoh: 1985
Wilayah pp1/pp2/serpong/bukit/reni

Contoh:
DAFTAR-James Munthe-pp1-1972#
DAFTAR-Maria Fatmitasari-pp2-1972#
DAFTAR-Ery Setiawan-bukit-1972#
DAFTAR-Florencia Irena-reni-1980#
DAFTAR-Titus Adi Prasetyo-serpong-1985#`

	InfoPemilihanHelp = `%sPemilihan pendeta di laksanakan secara Onsite dan Online

1. Pemilihan Onsite di Tanggal 28 September 2025 Jam 10:00 WIB (Setelah Ibadah) sampai Selesai
2. Pemilihan Online di Tanggal 25 September 2025 Jam 00:00 WIB sampai 27 September 2025 Jam 24:00 WIB

Untuk informasi lebih lanjut, 
silahkan menghubungi Majelis Wilayah atau Panitia`

	ResendVoteHelp = `%sBerikut ini surat suara Digital Anda : 

https://pemilihan.gkj-pamulang.org/%s

Simpan surat suara digital ini dengan aman.`

	NotYetRegistered = `Anda belum terdaftar di dalam pemilihan online,
Silahkan melakukan pendaftaran terlebih dahulu`

	AlreadyRegistered = `Anda sudah terdaftar sebelumnya dengan data:
Nama: %s
Wilayah: %s
Tahun Lahir: %s

Data tidak dapat diubah. 
Hubungi panitia jika ada kesalahan data.`
)

const (
	backHint = "\n\nKetik 0 untuk kembali ke menu utama"
)

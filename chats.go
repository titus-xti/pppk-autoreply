package main

var (
	OpeningGreeting = `Selamat datang di Asisten Virtual PPPK GKJP:\n
	1. Info Pemilihan\n
	2. Registrasi pemilihan Online\n
	3. Kirim ulang surat suara Digital\n\n
	Balas dengan angka: 1, 2 atau 3.`

	ErrorSaving = `Maaf, terjadi kesalahan saat menyimpan data. Silakan coba lagi.`

	NotRegistered = `Nomor anda belum terdaftar\n
	Silahkan menghubungi Majelis Wilayah atau Panitia`

	ErrorQuerying = `Maaf, terjadi kesalahan saat memeriksa data. Silakan coba lagi.`
	ErrorWilayah  = `Wilayah pelayanan tidak valid. Silahkan isi pp1/pp2/serpong/bukit/reni\n\n`
	ErrorDOB      = `Tahun lahir tidak valid. Silahkan isi tahun lahir 4 digit\n\n`

	SuccessRegister = `Terima kasih!\n
	Pendaftaran pemilihan online berhasil dengan data sebagai berikut:\n
	Nama: %s\n
	Wilayah: %s\n
	Tahun Lahir: %s\n\n
	Surat suara Digital: https://pemilihan.gkj-pamulang.org/%s\n\n
	Silahkan surat suara Digital ini dengan aman.\n\n
	Ketik '0' untuk kembali ke menu.`

	RegistrationHelp = `%sUntuk mendaftar pemilihan online, silahkan kirim pesan dengan format:\n
	DAFTAR-<nama lengkap jemaat>-<wilayah pelayanan>-<tahun lahir># \n\n
	Tahun Lahir 4 Digit, Contoh: 1985\n
	Wilayah pp1/pp2/serpong/bukit/reni\n\n
	Contoh:\n
	DAFTAR-James Munthe-pp1-1972#\n
	DAFTAR-Maria Fatmitasari-pp2-1972#\n
	DAFTAR-Ery Setiawan-bukit-1972#\n
	DAFTAR-Florencia Irena-reni-1980#\n
	DAFTAR-Titus Adi Prasetyo-serpong-1985#`

	InfoPemilihanHelp = `%sPemilihan pendeta di laksanakan secara Onsite dan Online \n
	Pemilihan Online di Tanggal 28 September 2025\n
	Pemilihan Onsite di Tanggal 28 September 2025\n
	Untuk informasi lebih lanjut, silahkan menghubungi Majelis Wilayah\n
	dan Panitia di nomor 081297898399`

	ResendVoteHelp = `%sBerikut ini surat suara Digital Anda : \n\n`

	AlreadyRegistered = `Anda sudah terdaftar sebelumnya dengan data:\n
	Nama: %s\n
	Wilayah: %s\n
	Tahun Lahir: %s\n\n
	Data tidak dapat diubah. \n\nHubungi panitia di nomor 081297898399 jika ada kesalahan data.`
)

const (
	backHint = "\n\nketik 0 untuk kembali ke menu utama"
)

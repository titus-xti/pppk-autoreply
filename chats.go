package main

var (
	OpeningGreeting = `Selamat datang di Chatbot Panitia Pemanggilan Pendeta Kedua GKJ Pamulang:

1. Informasi Pemilihan
2. Registrasi Pemilihan Online
3. Kirim ulang surat suara Digital

Balas dengan angka: 1, 2 atau 3.`

	ErrorSaving = `Maaf, terjadi kesalahan saat menyimpan data. Silakan coba lagi.`

	NotRegistered = `Nomor anda belum terdaftar,
Jika anda adalah warga dewasa GKJ Pamulang,
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

	RegistrationHelp = `Konfirmasi pendaftaran pemilihan online

Nama : %s
Wilayah : %s
Tahun Lahir : %s

Ketik "ya" untuk mendaftar pemilihan online`

	InfoPemilihanHelp = `%sPemilihan pendeta di laksanakan secara Onsite/Langsung dan Online

*1. Pemilihan Onsite/Langsung*
- Dilaksanakan pada Tanggal 28 September 2025 Jam 10:00 WIB (Setelah Ibadah) sampai Selesai
- Jemaat di harapkan hadir di Gereja, 30 menit sebelum Ibadah dimulai untuk mengisi daftar hadir dan mendapatkan surat suara
- Jemaat melakukan pencoblosan kartu suara dengan pilihan SETUJU atau TIDAK SETUJU

*2. Pemilihan Online*
- Diutamakan bagi Jemaat yang sedang Study luar kota, Tugas di luar Kota atau berhalangan hadir di Gereja pada saat pemilihan Onsite/Langsung Tanggal 28 September 2025 
- Jemaat terlebih dahulu melakukan pendaftaran pemilihan online, paling lambat 21 September 2025
- Satu nomor HP/WA untuk satu Jemaat
- Pastikan surat suara digital yang diterima disimpan dengan baik, dan tidak di pergunakan orang lain
- Jemaat bisa mulai memilih pada Tanggal 25 September 2025 Jam 00:00 WIB sampai 27 September 2025 Jam 24:00 WIB
- Jemaat memilih pada kartu suara digital dengan pilihan SETUJU atau TIDAK SETUJU

Untuk informasi lebih lanjut, 
silahkan menghubungi Majelis Wilayah atau Panitia`

	ResendVoteHelp = `%sBerikut ini surat suara Digital Anda : 

https://pemilihan.gkj-pamulang.org/%s

*Simpan surat suara digital ini dengan aman dan pastikan surat suara digital ini tidak dipergunakan orang lain*`

	NotYetRegistered = `Anda belum terdaftar di dalam pemilihan online,
Silahkan melakukan pendaftaran terlebih dahulu`

	AlreadyRegistered = `Anda sudah terdaftar dengan data sebagai berikut:

Nama: %s
Wilayah: %s
Tahun Lahir: %s

Data tidak dapat diubah. 
Hubungi Majelis wilayah atau Panitia jika ada kesalahan data.`
)

const (
	backHint = "\n\nKetik 0 untuk kembali ke menu utama\n\n*Semua pesan di jawab oleh System, mohon hanya mengirimkan pesan sesuai instruksi*\n\n*Terimakasih*"
)

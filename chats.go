package main

var (
	OpeningGreeting = `Selamat datang di Chatbot Panitia Pemanggilan Pendeta Kedua GKJ Pamulang:

1. Informasi Pemilihan
2. Registrasi Pemilihan Online
3. Kirim ulang surat suara Digital
4. Profil Calon Pendeta

Balas dengan angka: 1, 2, 3 atau 4.`

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

	InfoPemilihanHelp = `%sAnda terpilih sebagai jemaat GKJ Pamulang yang memiliki hak pilih/suara dalam pemilihan calon pendeta GKJ Pamulang.

Pemilihan dilaksanakan melalui pemungutan suara _(voting)_ yang akan dilaksanakan *pada hari Minggu tanggal 28 September 2025* mulai jam 10:00 WIB (atau setelah Ibadah minggu) sampai dengan selesai bertempat di GKJ Pamulang.

Jemaat GKJ Pamulang yang berhak mengikuti pemilihan adalah jemaat dewasa, yakni jemaat yang telah mengaku percaya/sidi; atau yang sudah baptis dewasa.

*Selain itu, jemaat simpatisan termasuk jemaat titipan juga berhak mengikuti pemilihan dengan syarat jemaat dimaksud telah mengaku percaya/sidi atau yang sudah baptis dewasa pada gereja lain; dan khusus untuk jemaat simpatisan dilengkapi dengan pernyataan keikutsertaaan untuk memilih dan pernyataan sudah sidi/baptis dewasa.*

Jemaat akan diminta untuk mencoblos surat suara dengan pilihan SETUJU atau TIDAK SETUJU Sdr. Pnt. Faisha Sudarlin, M.Th menjadi pendeta di GKJ Pamulang (catatan: penetapan seseorang menjadi pendeta terlebih dahulu diharuskan menjalani beberapa tahapan yang secara umum meliputi uji kelayakan calon pendeta sampai dengan penahbisan pendeta).

Berikut tata cara pemilihan:

*1. Pemilihan Langsung/on site*
- Jemaat diharapkan sudah hadir di Gereja 30 menit sebelum ibadah dimulai untuk mengisi daftar hadir dan mendapatkan surat suara.
- Setelah ibadah selesai, jemaat akan diminta panitia untuk maju ke bilik suara yang tersedia, melakukan pencoblosan dan memasukan surat suara ke dalam kotak suara yang tersedia.
- Jemaat dapat melihat langsung proses perhitungan suara atau melalui live streaming.

*2. Pemilihan On line*
- Diberlakukan bagi Jemaat yang sedang studi atau bertugas di luar kota, atau karena sebab lain sehingga berhalangan hadir pada hari pemilihan.
- Jemaat terlebih dahulu melakukan pendaftaran pemilihan online, paling lambat 21 September 2025.
- Satu nomor HP/WA hanya untuk satu Jemaat.
- Surat suara digital yang diterima agar disimpan dengan baik, dan tidak boleh di pergunakan orang lain.
- Jemaat bisa mulai memilih pada tanggal 25 September 2025 Jam 00:00 WIB sampai 27 September 2025 Jam 24:00 WIB.

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

	ProfilCalonPendeta = `*PROFIL CALON PENDETA GKJ PAMULANG*
 
1. Nama lengkap : Faisha Sudarlin
2. Usia : 30 tahun
3. Pendidikan terakhir: S2 / Magister Teologi
4. Domisili: Pamulang, Tangerang Selatan
5. Pekerjaan saat ini: Tenaga Pelayan Gerejawi di GKJ Pamulang dan Anggota Tim Inti Nyanyian Gereja di Yayasan Musik Gereja Indonesia

Sdr. Faisha Sudarlin yang akrab dengan panggilan mas Fa memulai tugas sebagai tenaga pelayan gerejawi di GKJ Pamulang sejak tanggal 10 Juni 2018, setelah itu diangkat sebagai karyawan tetap di GKJ Pamulang per 1 September 2020. Saat ini yang bersangkutan juga menjadi majelis gereja dengan jabatan Penatua membawahi bidang Kesaksian dan Pelayanan.

Di awal pelayanannya, mas Fa menjalankan tugas-tugas gerejawi antara lain: melatih musik bagi pemandu/pengiring lagu pujian; melatih paduan suara untuk beberapa kategorial seperti: Adiyuswa, KPR, dan jemaat dewasa/wilayah; termasuk bertugas sebagai pemandu/pengiring lagu pujian. Dalam perkembangannya, Mas Fa juga menjalankan tugas gerejawi lainnya, yakni sebagai pembawa firman baik dalam ibadah minggu atapun ibadah lainnya.

Dengan memperhatikan hasil jajak pendapat dengan jemaat terhadap pencalonan mas Fa sebagai pendeta GKJ Pamulang, maka berdasarkan Keputusan No.: *KEP-05/MG/GKJP/III/2025, tanggal 25 Maret 2025*, Majelis telah memutuskan Sdr. Pnt. Faisha Sudarlin, MTh. Sebagai calon tunggal untuk calon Pendeta kedua GKJ Pamulang.

Atas dasar keputusan tersebut, GKJ Pamulang telah menjalankan kegiatan pemanggilan pendeta, meliputi:

a. Proses pemanggilan calon pendeta (bulan April 2025)
b. Proses asesmen calon pendeta (bulan Mei 2025) dan
c. Masa orientasi / pengenalan calon pendeta (bulan Mei s.d Agustus 2025).
Adapun kegiatan yang akan dijalankan di bulan September 2025 adalah pemilihan calon pendeta melalui voting, dalam hal ini jemaat akan dimintakan suaranya dalam memutuskan *Setuju* atau *Tidak Setuju* bila mas Fa menjadi Pendeta di GKJ Pamulang.
`
)

const (
	backHint = "\n\nKetik 0 untuk kembali ke menu utama\n\n*Semua pesan di jawab oleh System, mohon hanya mengirimkan pesan sesuai instruksi*\n\n*Terimakasih*"
)

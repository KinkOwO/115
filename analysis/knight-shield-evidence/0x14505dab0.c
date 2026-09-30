__int64 __fastcall sub_14505DAB0(__int64 a1)
{
  __int64 v2; // rax
  __int64 result; // rax
  __int64 v4; // rcx
  __int64 v5; // rsi
  __int64 v6; // rax
  __int64 v7; // rcx
  __int64 v8; // rax
  __int64 v9; // rcx
  __int64 v10; // rax
  __int64 v11; // rcx
  __int64 v12; // rax
  __int64 v13; // rcx
  __int64 v14; // rax
  __int64 v15; // rcx
  __int64 v16; // rax
  __int64 v17; // rcx
  __int64 v18; // rax
  __int64 v19; // rcx
  __int64 v20; // rax
  __int64 v21; // rcx
  __int64 v22; // rax
  __int64 v23; // rcx
  __int64 v24; // rax
  __int64 v25; // rcx
  __int64 v26; // rax
  __int64 v27; // rcx
  __int64 v28; // rax
  __int64 v29; // rcx
  __int64 v30; // rax
  __int64 v31; // rcx
  __int64 v32; // rax
  int i; // ebx
  unsigned int v34; // edi
  __int64 v35; // rcx
  __int64 v36; // rax

  v2 = sub_145EFAFB0(); /*0x14505dabd*/
  result = sub_1450BE540(a1: v2); /*0x14505dac5*/
  v5 = result; /*0x14505daca*/
  if ( result != 0 ) /*0x14505dad0*/
  {
    if ( *(_BYTE *)(a1 + 120) == 1 ) /*0x14505dada*/
    {
      v6 = sub_146D74000(a1: v4); /*0x14505dae0*/
      sub_146D746E0(a1: v6, a2: 19); /*0x14505daed*/
      v8 = sub_146D74000(a1: v7); /*0x14505daf2*/
      sub_146D75CC0(a1: v8, a2: *(unsigned __int8 *)(a1 + 400)); /*0x14505db01*/
      v10 = sub_146D74000(a1: v9); /*0x14505db06*/
      sub_146D76180(a1: v10, a2: *(unsigned __int16 *)(a1 + 404)); /*0x14505db15*/
      v12 = sub_146D74000(a1: v11); /*0x14505db1a*/
      sub_146D75CE0(a1: v12, a2: *(unsigned int *)(a1 + 408)); /*0x14505db28*/
      v14 = sub_146D74000(a1: v13); /*0x14505db2d*/
      sub_146D75CE0(a1: v14, a2: 0); /*0x14505db37*/
      v16 = sub_146D74000(a1: v15); /*0x14505db3c*/
      sub_146D75CC0(a1: v16, a2: *(unsigned __int8 *)(a1 + 412)); /*0x14505db4b*/
      v18 = sub_146D74000(a1: v17); /*0x14505db50*/
      sub_146D76180(a1: v18, a2: *(unsigned __int16 *)(a1 + 416)); /*0x14505db5f*/
      v20 = sub_146D74000(a1: v19); /*0x14505db64*/
      sub_146D75CE0(a1: v20, a2: *(unsigned int *)(a1 + 420)); /*0x14505db72*/
      v22 = sub_146D74000(a1: v21); /*0x14505db77*/
      sub_146D75CE0(a1: v22, a2: 0); /*0x14505db81*/
      v24 = sub_146D74000(a1: v23); /*0x14505db86*/
      sub_146D75CE0(a1: v24, a2: 0xFFFFFFFFLL); /*0x14505db93*/
      v26 = sub_146D74000(a1: v25); /*0x14505db98*/
      sub_146D75CC0(a1: v26, a2: 0); /*0x14505dba2*/
      v28 = sub_146D74000(a1: v27); /*0x14505dba7*/
      sub_146D75CC0(a1: v28, a2: 0); /*0x14505dbb1*/
      v30 = sub_146D74000(a1: v29); /*0x14505dbb6*/
      sub_146D75CC0(a1: v30, a2: 0); /*0x14505dbc0*/
    }
    else
    {
      v32 = sub_146D74000(a1: v4); /*0x14505dbd1*/
      sub_146D746E0(a1: v32, a2: 649); /*0x14505dbde*/
      for ( i = 0; i < 5; ++i ) /*0x14505dbe3*/
      {
        v34 = sub_144BE1890(a1: v5, a2: (unsigned int)i); /*0x14505dbef*/
        v36 = sub_146D74000(a1: v35); /*0x14505dbf1*/
        sub_146D75CE0(a1: v36, a2: v34); /*0x14505dbfb*/
      }
    }
    result = sub_146D75AF0(a1: v31); /*0x14505dc11*/
    *(_BYTE *)(a1 + 120) = 0; /*0x14505dc16*/
  }
  return result; /*0x14505dc1a*/
}

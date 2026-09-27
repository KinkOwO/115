// fn_noti_call_1456b8ab0

// sub_1456B8AB0
void __fastcall sub_1456B8AB0(unsigned __int16 a1, __int64 a2)
{
  char v3; // r15
  signed __int8 v4; // r13
  __int64 v5; // r12
  __int64 v6; // rdi
  int v7; // ecx
  int v8; // edx
  __int64 v9; // rax
  __int64 v10; // rax
  __int64 **v11; // rdx
  __int64 *i; // rcx
  __int64 v13; // rcx
  __int64 j; // rax
  __int16 v15; // bx
  __int64 *v16; // rdx
  __int128 *v17; // r10
  char v18; // si
  char v19; // r14
  __int64 v20; // r9
  bool v21; // cc
  char v22; // cl
  bool v23; // cc
  char v24; // cl
  int v25; // edi
  __int64 v26; // rdx
  __int64 v27; // rdi
  __int128 *v28; // rcx
  __int64 v29; // r8
  char v30; // al
  __int64 v31; // rax
  __int64 v32; // rax
  __int64 v33; // rcx
  __int64 v34; // rax
  __int64 v35; // rdx
  __int64 v36; // rcx
  __int64 v37; // rax
  __int64 v38; // rdx
  __int64 v39; // rcx
  __int64 v40; // rax
  __int64 v41; // rcx
  __int64 v42; // rax
  __int64 v43; // rdx
  __int64 v44; // rdx
  __int64 v45; // rcx
  __int64 v46; // rax
  int v47; // [rsp+20h] [rbp-60h] BYREF
  __int64 v48; // [rsp+28h] [rbp-58h]
  __int64 v49; // [rsp+30h] [rbp-50h]
  __int128 **v50; // [rsp+38h] [rbp-48h]
  __int128 **v51; // [rsp+40h] [rbp-40h]
  __int128 *v52; // [rsp+48h] [rbp-38h] BYREF
  __int64 v53; // [rsp+50h] [rbp-30h]
  __int128 **v54; // [rsp+60h] [rbp-20h] BYREF
  __int128 **v55; // [rsp+68h] [rbp-18h]
  __int16 v56; // [rsp+C8h] [rbp+48h] BYREF
  __int16 v57; // [rsp+D0h] [rbp+50h]
  int v58; // [rsp+D8h] [rbp+58h] BYREF

  v49 = -2;
  if ( *(_BYTE *)a2 )
    v3 = *(_BYTE *)a2 == 1;
  else
    v3 = 2;
  v4 = sub_1401BDF80((unsigned __int8)v3, a1);
  if ( v4 )
  {
    v5 = a2 + 16;
    v6 = sub_146EA1540();
    LOWORD(v6) = -16 - a2 + v6;
    v48 = v6;
    if ( !(unsigned __int8)sub_146EA40F0(4) )
    {
      sub_146EA0BA0(&v58);
      v7 = *(_DWORD *)(a2 + 3) - 17;
      v8 = (__int16)v6 + 5;
      if ( v7 >= v8 )
      {
        while ( !*(_BYTE *)(v7 + v5) )
        {
          if ( --v7 < v8 )
            goto LABEL_11;
        }
        v58 = *(_DWORD *)(v7 + v5 - 4);
        LOWORD(v48) = v7 - 4;
      }
LABEL_11:
      LOBYTE(v57) = v3;
      HIBYTE(v57) = v4;
      if ( dword_14E66FDF0 > *(_DWORD *)(*((_QWORD *)NtCurrentTeb()->ThreadLocalStoragePointer
                                         + (unsigned int)dword_14F3BEE58)
                                       + 420620LL) )
      {
        sub_148860450(&dword_14E66FDF0);
        if ( dword_14E66FDF0 == -1 )
        {
          xmmword_14E66FDE0 = 0;
          v46 = sub_146E8BA20(40);
          *(_QWORD *)v46 = v46;
          *(_QWORD *)(v46 + 8) = v46;
          *(_QWORD *)(v46 + 16) = v46;
          *(_WORD *)(v46 + 24) = 257;
          *(_QWORD *)&xmmword_14E66FDE0 = v46;
          sub_14885FFE8(sub_149034110);
          sub_1488603F0(&dword_14E66FDF0);
        }
      }
      v50 = &v52;
      v52 = 0;
      v53 = 0;
      v54 = &v52;
      v55 = &v52;
      v9 = sub_146E8BA20(40);
      *(_QWORD *)v9 = v9;
      *(_QWORD *)(v9 + 8) = v9;
      *(_QWORD *)(v9 + 16) = v9;
      *(_WORD *)(v9 + 24) = 257;
      v52 = (__int128 *)v9;
      v10 = sub_1401DBAA0(&v52, *(_QWORD *)(xmmword_14E66FDE0 + 8), v9, (unsigned __int8)v56);
      *((_QWORD *)v52 + 1) = v10;
      v53 = *((_QWORD *)&xmmword_14E66FDE0 + 1);
      v11 = (__int64 **)*((_QWORD *)v52 + 1);
      if ( *((_BYTE *)v11 + 25) )
      {
        *(_QWORD *)v52 = v52;
        *((_QWORD *)v52 + 2) = v52;
      }
      else
      {
        for ( i = *v11; !*((_BYTE *)i + 25); i = (__int64 *)*i )
          v11 = (__int64 **)i;
        *(_QWORD *)v52 = v11;
        v13 = *((_QWORD *)v52 + 1);
        for ( j = *(_QWORD *)(v13 + 16); !*(_BYTE *)(j + 25); j = *(_QWORD *)(j + 16) )
          v13 = j;
        *((_QWORD *)v52 + 2) = v13;
      }
      v55 = 0;
      v51 = &v52;
      v15 = v57;
      v56 = v57;
      v16 = (__int64 *)*((_QWORD *)v52 + 1);
      v17 = v52;
      v18 = 1;
      v19 = -1;
      v20 = HIBYTE(v57);
      while ( !*((_BYTE *)v16 + 25) )
      {
        v21 = *((_BYTE *)v16 + 28) < (unsigned __int8)v57;
        if ( *((_BYTE *)v16 + 28) == (_BYTE)v57
          && (v21 = *((_BYTE *)v16 + 29) < SHIBYTE(v57), *((_BYTE *)v16 + 29) == HIBYTE(v57)) )
        {
          v22 = 0;
        }
        else
        {
          v22 = 1;
          if ( v21 )
            v22 = -1;
        }
        if ( v22 >= 0 )
        {
          v17 = (__int128 *)v16;
          v16 = (__int64 *)*v16;
        }
        else
        {
          v16 = (__int64 *)v16[2];
        }
      }
      if ( *((_BYTE *)v17 + 25) )
        goto LABEL_37;
      v23 = (char)v57 < *((_BYTE *)v17 + 28);
      if ( (_BYTE)v57 == *((_BYTE *)v17 + 28)
        && (v23 = SHIBYTE(v57) < *((_BYTE *)v17 + 29), HIBYTE(v57) == *((_BYTE *)v17 + 29)) )
      {
        v24 = 0;
      }
      else
      {
        v24 = 1;
        if ( v23 )
          v24 = -1;
      }
      if ( v24 < 0 || v17 == v52 )
LABEL_37:
        *(_DWORD *)(*(_QWORD *)sub_1456B5980(&v52, &v54, &v56, HIBYTE(v57)) + 32LL) = 0;
      v25 = *(_DWORD *)(*(_QWORD *)sub_1456B5980(&v52, &v54, &v56, v20) + 32LL);
      sub_140150890(&v52, &v52, *((_QWORD *)v52 + 1));
      sub_146E9F3A0(v52, 40);
      if ( v25 <= 40 && (unsigned __int8)sub_1401BE160(v5, (unsigned __int16)v48, &v47) && v47 != v58 )
      {
        v27 = xmmword_14E66FDE0;
        v28 = *(__int128 **)(xmmword_14E66FDE0 + 8);
        v54 = (__int128 **)v28;
        LODWORD(v55) = 0;
        v29 = xmmword_14E66FDE0;
        while ( !*((_BYTE *)v28 + 25) )
        {
          v54 = (__int128 **)v28;
          if ( *((_BYTE *)v28 + 28) == v3 )
          {
            if ( *((_BYTE *)v28 + 29) == v4 )
            {
              v30 = 0;
            }
            else
            {
              v30 = -1;
              if ( *((char *)v28 + 29) >= v4 )
                v30 = 1;
            }
          }
          else
          {
            v30 = 1;
            if ( *((char *)v28 + 28) < v3 )
              v30 = -1;
          }
          if ( v30 >= 0 )
          {
            LODWORD(v55) = 1;
            v29 = (__int64)v28;
            v28 = *(__int128 **)v28;
          }
          else
          {
            LODWORD(v55) = 0;
            v28 = (__int128 *)*((_QWORD *)v28 + 2);
          }
        }
        if ( *(_BYTE *)(v29 + 25) )
          goto LABEL_65;
        if ( v3 == *(_BYTE *)(v29 + 28) )
        {
          if ( v4 == *(_BYTE *)(v29 + 29) )
          {
            v19 = 0;
          }
          else if ( v4 >= *(char *)(v29 + 29) )
          {
            v19 = 1;
          }
        }
        else
        {
          if ( v3 < *(char *)(v29 + 28) )
            v18 = -1;
          v19 = v18;
        }
        if ( v19 < 0 )
        {
LABEL_65:
          if ( *((_QWORD *)&xmmword_14E66FDE0 + 1) == 0x666666666666666LL )
            sub_14014F360(v28, v26);
          v52 = &xmmword_14E66FDE0;
          v53 = 0;
          v31 = sub_146E8BA20(40);
          *(_WORD *)(v31 + 28) = v15;
          *(_DWORD *)(v31 + 32) = 0;
          *(_QWORD *)v31 = v27;
          *(_QWORD *)(v31 + 8) = v27;
          *(_QWORD *)(v31 + 16) = v27;
          *(_WORD *)(v31 + 24) = 0;
          v53 = 0;
          v29 = sub_14014F0E0(&xmmword_14E66FDE0, &v54, v31);
        }
        ++*(_DWORD *)(v29 + 32);
        v32 = sub_146D74000(v28);
        sub_146D746E0(v32, 283);
        v34 = sub_146D74000(v33);
        LOBYTE(v35) = 1;
        sub_146D75CC0(v34, v35);
        v37 = sub_146D74000(v36);
        LOBYTE(v38) = 2;
        sub_146D75CC0(v37, v38);
        v40 = sub_146D74000(v39);
        sub_146D75CC0(v40, (unsigned __int8)v4);
        v42 = sub_146D74000(v41);
        LOBYTE(v43) = 1;
        sub_146D75CC0(v42, v43);
        sub_146D75AF0(v45, v44);
      }
    }
  }
}


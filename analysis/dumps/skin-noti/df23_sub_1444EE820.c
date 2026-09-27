// sub_1444EE820

__int64 __fastcall sub_1444EE820(__int64 a1)
{
  unsigned int *v2; // rsi
  __int64 v3; // rcx
  __int64 v4; // r8
  int v5; // edi
  unsigned int *v6; // rbx
  __int64 v7; // rdx
  __int64 v8; // rax
  unsigned int v9; // eax
  unsigned int *v10; // r14
  __int64 v11; // r15
  __int64 v12; // r8
  unsigned int v13; // ebx
  __int64 v14; // rax
  unsigned int *v15; // rax
  __int64 v16; // rdx
  __int64 v17; // rax
  unsigned int v18; // eax
  __int64 v19; // rax
  __int64 i; // rbx
  unsigned int v21; // eax
  __int64 v22; // rcx
  __int64 v23; // rcx
  __int64 v24; // rbx
  __int64 v25; // rax
  __int64 *v26; // r8
  __int64 *v27; // rcx
  __int64 *v28; // rdx
  __int64 *v29; // r8
  __int64 *v30; // rax
  __int64 *v31; // rcx
  __int64 result; // rax
  __int64 v33; // rcx
  unsigned __int64 v34; // rdx
  unsigned int v35; // [rsp+28h] [rbp-99h] BYREF
  __int128 v36; // [rsp+30h] [rbp-91h] BYREF
  unsigned int *v37; // [rsp+40h] [rbp-81h]
  int v38; // [rsp+48h] [rbp-79h]
  __int64 v39; // [rsp+50h] [rbp-71h]
  int v40; // [rsp+58h] [rbp-69h] BYREF
  _BYTE v41[24]; // [rsp+60h] [rbp-61h] BYREF
  _BYTE v42[16]; // [rsp+78h] [rbp-49h] BYREF
  _DWORD v43[24]; // [rsp+88h] [rbp-39h] BYREF

  v39 = -2;
  v38 = 0;
  v36 = 0u;
  v37 = 0;
  v2 = 0;
  v43[0] = 10;
  memset(&v43[1], 0, 84);
  sub_146EA0BE0(v43, 88);
  if ( !v43[1] )
  {
    switch ( v43[0] )
    {
      case 0:
        v5 = 0;
        v6 = &v43[2];
        break;
      case 1:
        v10 = &v43[2];
        v11 = 20;
        do
        {
          if ( sub_145EFAFB0(v3) )
          {
            v13 = *v10;
            v14 = sub_145EFAFB0(v3);
            v15 = (unsigned int *)(*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v14 + 7656LL))(v14);
            *v10 = sub_1473A1120(a1 + 1328, *v15, v13);
          }
          v16 = *v10;
          if ( (_DWORD)v16 )
          {
            LOBYTE(v12) = 1;
            v17 = sub_140283D60(qword_14E683BF8, v16, v12);
            if ( !v17 || *(_DWORD *)(v17 + 8) != 1 || *(_DWORD *)(v17 + 12) != 3 )
            {
              v18 = *v10;
              v35 = *v10;
              if ( v2 == v37 )
              {
                sub_140154010(&v36, v2, &v35);
                v2 = (unsigned int *)*((_QWORD *)&v36 + 1);
              }
              else
              {
                *v2++ = v18;
                *((_QWORD *)&v36 + 1) = v2;
              }
            }
          }
          ++v10;
          --v11;
        }
        while ( v11 );
        goto LABEL_37;
      case 2:
        v35 = v43[2];
        sub_140154010(&v36, 0, &v35);
        v19 = sub_143C61150();
        sub_1447EF2F0(v19, v43[2]);
        goto LABEL_36;
      case 3:
        for ( i = 0; i < 4; ++i )
        {
          v21 = v43[i + 2];
          v35 = v21;
          if ( v2 == v37 )
          {
            sub_140154010(&v36, v2, &v35);
            v2 = (unsigned int *)*((_QWORD *)&v36 + 1);
          }
          else
          {
            *v2++ = v21;
            *((_QWORD *)&v36 + 1) = v2;
          }
        }
        goto LABEL_37;
      case 4:
        v35 = v43[2];
        sub_140154010(&v36, 0, &v35);
        if ( sub_145EFAFB0(v22) )
        {
          v24 = sub_145EFAFB0(v23);
          sub_145D87890(v24, v43[2]);
          sub_145D1D810(v24);
        }
        goto LABEL_36;
      case 6:
        v35 = v43[2];
        sub_140154010(&v36, 0, &v35);
        v25 = sub_143C61150();
        sub_142581F20(v25, v43[2]);
        goto LABEL_36;
      case 7:
      case 8:
        v35 = v43[2];
        sub_140154010(&v36, 0, &v35);
LABEL_36:
        v2 = (unsigned int *)*((_QWORD *)&v36 + 1);
        goto LABEL_37;
      default:
LABEL_37:
        v26 = *(__int64 **)(a1 + 296);
        v27 = (__int64 *)v26[1];
        v28 = v26;
        while ( !*((_BYTE *)v27 + 25) )
        {
          if ( *((_DWORD *)v27 + 8) >= v43[0] )
          {
            v28 = v27;
            v27 = (__int64 *)*v27;
          }
          else
          {
            v27 = (__int64 *)v27[2];
          }
        }
        if ( *((_BYTE *)v28 + 25) || v43[0] < *((_DWORD *)v28 + 8) || v28 == v26 )
        {
          v40 = v43[0];
          sub_1401550D0(v41, &v36);
          v38 = 1;
          sub_1401844D0(a1 + 296, v42, &v40);
          v38 = 0;
          sub_1401574A0(v41);
        }
        else
        {
          v28[6] = v28[5];
          sub_143D855D0(v28 + 5, v36, v2);
        }
        goto LABEL_58;
    }
    while ( 1 )
    {
      v7 = *v6;
      if ( (_DWORD)v7 )
      {
        LOBYTE(v4) = 1;
        v8 = sub_140283D60(qword_14E683BF8, v7, v4);
        if ( v8 && *(_DWORD *)(v8 + 12) == 2 )
        {
          sub_1444E8ED0(a1, (unsigned int)v43[v5 + 2]);
          goto LABEL_58;
        }
        v9 = *v6;
        v35 = *v6;
        if ( v2 == v37 )
        {
          sub_140154010(&v36, v2, &v35);
          v2 = (unsigned int *)*((_QWORD *)&v36 + 1);
        }
        else
        {
          *v2++ = v9;
          *((_QWORD *)&v36 + 1) = v2;
        }
      }
      ++v5;
      ++v6;
      if ( v5 >= 20 )
        goto LABEL_37;
    }
  }
  if ( v43[1] == 4 && !v43[0] )
  {
    v35 = v43[2];
    v29 = *(__int64 **)(a1 + 312);
    v30 = (__int64 *)v29[1];
    v31 = v29;
    while ( !*((_BYTE *)v30 + 25) )
    {
      if ( *((_DWORD *)v30 + 7) >= v43[2] )
      {
        v31 = v30;
        v30 = (__int64 *)*v30;
      }
      else
      {
        v30 = (__int64 *)v30[2];
      }
    }
    if ( !*((_BYTE *)v31 + 25) && v43[2] >= *((_DWORD *)v31 + 7) && v31 != v29 )
      sub_1403A7590(a1 + 312, &v35);
  }
LABEL_58:
  result = sub_1444F1EE0(a1, v43[0]);
  v33 = v36;
  if ( (_QWORD)v36 )
  {
    v34 = ((unsigned __int64)v37 - v36) & 0xFFFFFFFFFFFFFFFCuLL;
    if ( v34 >= 0x1000 )
    {
      v34 += 39LL;
      v33 = *(_QWORD *)(v36 - 8);
      if ( (unsigned __int64)(v36 - v33 - 8) > 0x1F )
        sub_148AAF304(v33, v34);
    }
    result = sub_146E9F3A0(v33, v34);
    v36 = 0;
    v37 = 0;
  }
  return result;
}

